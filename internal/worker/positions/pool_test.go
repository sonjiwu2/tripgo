package positions_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/worker/positions"
)

const (
	queued    = "queued"
	duplicate = "duplicate"
	dropped   = "dropped"
	success   = "success"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestScanReadsThreePages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const batch, count = 100, 250
		f := newPool(t, 1, count, batch)
		rows := quietTrips(count)
		at := time.Now()
		gomock.InOrder(
			f.page(at, domaintrip.Quiet{}, rows[:100]),
			f.page(at, rows[99], rows[100:200]),
			f.page(at, rows[199], rows[200:]),
		)
		f.checks.EXPECT().Add(gomock.Any(), queued).Times(count)
		f.rows.EXPECT().Record(gomock.Any(), int64(count))
		f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).Times(count)
		f.requests.EXPECT().Add(gomock.Any(), success).Times(count)
		f.tick(at)
		f.stop(t)
	})
}

func TestScanStopsAfterFirstDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const batch, capacity = 100, 50
		f := newPool(t, 1, capacity, batch)
		release := f.blockWorker()
		rows := quietTrips(250)
		at := time.Now()
		f.page(at, domaintrip.Quiet{}, rows[:batch])
		f.checks.EXPECT().Add(gomock.Any(), queued).Times(capacity)
		f.checks.EXPECT().Add(gomock.Any(), dropped)
		f.rows.EXPECT().Record(gomock.Any(), int64(batch))
		f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).Times(capacity)
		f.requests.EXPECT().Add(gomock.Any(), success).Times(capacity)
		f.tick(at)
		if got := f.pool.QueueSize(); got != capacity {
			t.Fatalf("queue size %d, want %d", got, capacity)
		}
		close(release)
		f.stop(t)
	})
}

func TestTickDoesNotDuplicateQueuedOrRunningTrip(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 3, 100)
		item := quietTrips(2)
		release := make(chan struct{})
		f.push.EXPECT().RequestPosition(gomock.Any(), item[0].DriverID, item[0].ID).
			DoAndReturn(func(context.Context, string, string) error {
				<-release
				return nil
			})
		f.push.EXPECT().RequestPosition(gomock.Any(), item[1].DriverID, item[1].ID)
		f.requests.EXPECT().Add(gomock.Any(), success).Times(2)
		at := time.Now()
		f.expectTick(at, item, queued)
		f.expectTick(at, item, duplicate)
		if got := f.pool.QueueSize(); got != 1 {
			t.Fatalf("queue size %d, want 1", got)
		}
		close(release)
		f.stop(t)
	})
}

func TestPriorityAcrossTicksAndEviction(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 3, 7)
		release := f.blockWorker()
		items := quietTrips(6)
		at := time.Now()
		f.expectTick(at, []domaintrip.Quiet{items[5], items[4]}, queued)
		f.expectTick(at, []domaintrip.Quiet{items[2]}, queued)

		f.checks.EXPECT().Add(gomock.Any(), dropped)
		f.expectTick(at, []domaintrip.Quiet{items[0]}, queued)
		f.expectTick(at, []domaintrip.Quiet{items[5]}, dropped)
		gomock.InOrder(
			f.push.EXPECT().RequestPosition(gomock.Any(), items[0].DriverID, items[0].ID),
			f.push.EXPECT().RequestPosition(gomock.Any(), items[2].DriverID, items[2].ID),
			f.push.EXPECT().RequestPosition(gomock.Any(), items[4].DriverID, items[4].ID),
		)
		f.requests.EXPECT().Add(gomock.Any(), success).Times(3)
		close(release)
		f.stop(t)
	})
}

func TestForgetRemovesQueuedTrip(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 1, 100)
		release := f.blockWorker()
		items := quietTrips(2)
		at := time.Now()
		f.expectTick(at, items[:1], queued)
		f.requests.EXPECT().Add(gomock.Any(), "cancelled")
		f.pool.Forget(context.Background(), items[0].ID)
		if got := f.pool.QueueSize(); got != 0 {
			t.Fatalf("queue size %d, want 0", got)
		}
		f.expectTick(at, items[1:], queued)
		f.push.EXPECT().RequestPosition(gomock.Any(), items[1].DriverID, items[1].ID)
		f.requests.EXPECT().Add(gomock.Any(), success)
		close(release)
		f.stop(t)
	})
}

func TestForgetDuringReadSkipsOldSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 1, 100)
		items := quietTrips(1)
		at := time.Now()
		release := make(chan struct{})
		f.src.EXPECT().Quiet(gomock.Any(), at, time.Minute, domaintrip.Quiet{}, 100).
			DoAndReturn(func(context.Context, time.Time, time.Duration, domaintrip.Quiet, int) ([]domaintrip.Quiet, error) {
				<-release
				return items, nil
			})
		f.rows.EXPECT().Record(gomock.Any(), int64(1))
		f.tick(at)
		f.pool.Forget(context.Background(), items[0].ID)
		close(release)
		synctest.Wait()
		f.stop(t)
	})
}

func TestWorkersStayWithinLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const workers, jobs = 3, 7
		f := newPool(t, workers, jobs, 100)
		release := make(chan struct{})
		f.push.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(context.Context, string, string) error {
				<-release
				return nil
			}).Times(jobs)
		f.requests.EXPECT().Add(gomock.Any(), success).Times(jobs)
		f.expectTick(time.Now(), quietTrips(jobs), queued)
		if busy, waiting := f.pool.WorkersBusy(), f.pool.QueueSize(); busy != workers || waiting != jobs-workers {
			t.Fatalf("busy %d, queued %d", busy, waiting)
		}
		close(release)
		f.stop(t)
	})
}

func TestPushErrorDoesNotStopWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 2, 100)
		items := quietTrips(2)
		gomock.InOrder(
			f.push.EXPECT().RequestPosition(gomock.Any(), items[0].DriverID, items[0].ID).Return(errors.New("push недоступен")),
			f.push.EXPECT().RequestPosition(gomock.Any(), items[1].DriverID, items[1].ID),
		)
		f.requests.EXPECT().Add(gomock.Any(), "error")
		f.requests.EXPECT().Add(gomock.Any(), success)
		f.expectTick(time.Now(), items, queued)
		f.stop(t)
	})
}

func TestShutdownCancelsScannerAndDrainsWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 2, 100)
		release := f.blockWorker()
		items := quietTrips(1)
		at := time.Now()
		f.expectTick(at, items, queued)
		f.src.EXPECT().Quiet(gomock.Any(), at, time.Minute, domaintrip.Quiet{}, 100).
			DoAndReturn(func(ctx context.Context, _ time.Time, _ time.Duration, _ domaintrip.Quiet, _ int) ([]domaintrip.Quiet, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			})
		f.rows.EXPECT().Record(gomock.Any(), int64(0))
		f.tick(at)
		f.push.EXPECT().RequestPosition(gomock.Any(), items[0].DriverID, items[0].ID)
		f.requests.EXPECT().Add(gomock.Any(), success)
		stopped := make(chan error, 1)
		go func() { stopped <- f.pool.Stop(context.Background()) }()
		synctest.Wait()
		select {
		case err := <-stopped:
			t.Fatalf("stopped before draining: %v", err)
		default:
		}
		close(release)
		if err := <-stopped; err != nil {
			t.Fatal(err)
		}
	})
}

func TestShutdownDeadlineCancelsPush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 1, 100)
		items := quietTrips(1)
		f.push.EXPECT().RequestPosition(gomock.Any(), items[0].DriverID, items[0].ID).
			DoAndReturn(func(ctx context.Context, _, _ string) error {
				<-ctx.Done()
				return ctx.Err()
			})
		f.requests.EXPECT().Add(gomock.Any(), "error")
		f.expectTick(time.Now(), items, queued)
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		if err := f.pool.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stop error %v", err)
		}
	})
}

func TestScanTicksDoNotOverlap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newPool(t, 1, 2, 100)
		hold := make(chan struct{})
		at := time.Now()
		f.src.EXPECT().Quiet(gomock.Any(), at, time.Minute, domaintrip.Quiet{}, 100).
			DoAndReturn(func(context.Context, time.Time, time.Duration, domaintrip.Quiet, int) ([]domaintrip.Quiet, error) {
				<-hold
				return nil, nil
			})
		f.rows.EXPECT().Record(gomock.Any(), int64(0))
		f.tick(at)
		select {
		case f.ticks <- at:
			t.Fatal("second tick was consumed during the first scan")
		default:
		}
		close(hold)
		f.stop(t)
	})
}

type fixture struct {
	pool     *positions.Pool
	src      *MockSource
	push     *MockPusher
	checks   *MockResultCounter
	requests *MockResultCounter
	rows     *MockRowsRecorder
	ticks    chan time.Time
	batch    int
}

func newPool(t *testing.T, workers, capacity, batch int) fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := fixture{
		src: NewMockSource(ctrl), push: NewMockPusher(ctrl),
		checks: NewMockResultCounter(ctrl), requests: NewMockResultCounter(ctrl),
		rows: NewMockRowsRecorder(ctrl), ticks: make(chan time.Time), batch: batch,
	}
	timing := NewMockDurationRecorder(ctrl)
	timing.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	f.pool = positions.New(
		slog.New(slog.DiscardHandler), f.src, f.push,
		positions.Config{
			Interval: time.Hour, Stale: time.Minute,
			Workers: workers, Queue: capacity, BatchSize: batch, PushTimeout: time.Second,
			Clock: func(time.Duration) (<-chan time.Time, func()) {
				return f.ticks, func() {}
			},
		},
		f.checks, f.requests, timing, f.rows,
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f.pool.Start(ctx)
	return f
}

func (f fixture) page(at time.Time, after domaintrip.Quiet, rows []domaintrip.Quiet) *gomock.Call {
	return f.src.EXPECT().Quiet(gomock.Any(), at, time.Minute, after, f.batch).Return(rows, nil)
}

func (f fixture) tick(at time.Time) {
	f.ticks <- at
	synctest.Wait()
}

func (f fixture) expectTick(at time.Time, items []domaintrip.Quiet, result string) {
	f.page(at, domaintrip.Quiet{}, items)
	f.checks.EXPECT().Add(gomock.Any(), result).Times(len(items))
	f.rows.EXPECT().Record(gomock.Any(), int64(len(items)))
	f.tick(at)
}

func (f fixture) blockWorker() chan struct{} {
	item := quietTrips(1)[0]
	release := make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), item.DriverID, item.ID).
		DoAndReturn(func(context.Context, string, string) error {
			<-release
			return nil
		})
	f.requests.EXPECT().Add(gomock.Any(), success)
	f.expectTick(time.Now(), []domaintrip.Quiet{item}, queued)
	return release
}

func (f fixture) stop(t *testing.T) {
	t.Helper()
	if err := f.pool.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func quietTrips(count int) []domaintrip.Quiet {
	rows := make([]domaintrip.Quiet, 0, count)
	oldest := time.Now().Add(-time.Hour)
	for i := range count {
		rows = append(rows, domaintrip.Quiet{
			ID: uuid.NewString(), DriverID: uuid.NewString(),
			Since: oldest.Add(time.Duration(i) * time.Second),
		})
	}
	return rows
}

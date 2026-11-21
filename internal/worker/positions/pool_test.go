package positions_test

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/goleak"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/worker/positions"
)

const queued = "queued"

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestTickDoesNotDuplicateQueuedTrip(t *testing.T) {
	block := make(chan struct{})
	push := newGate(block)
	src := newSrc([]domaintrip.Quiet{quietTrip()})
	checks := &memCount{}
	ticks := make(chan time.Time)
	pool := newPool(t, src, push, checks, &memCount{}, 1, 4, ticks)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(root)
	defer func() { _ = pool.Stop(context.Background()) }()

	ticks <- time.Now()
	<-push.entered
	ticks <- time.Now()
	<-src.called
	<-src.called
	if checks.get("queued") != 1 || checks.get("duplicate") != 1 {
		t.Fatalf("queued %d duplicate %d", checks.get("queued"), checks.get("duplicate"))
	}
	close(block)
}

func TestForgetSkipsPush(t *testing.T) {
	block := make(chan struct{})
	push := newGate(block)
	first := quietTrip()
	second := quietTrip()
	third := quietTrip()
	ticks := make(chan time.Time)
	pool := newPool(t, newSrc(nil), push, &memCount{}, &memCount{}, 1, 4, ticks)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(root)
	defer func() { _ = pool.Stop(context.Background()) }()

	if pool.Put(context.Background(), first, trace.Link{}) != queued {
		t.Fatal("first not queued")
	}
	<-push.entered
	if pool.Put(context.Background(), second, trace.Link{}) != queued {
		t.Fatal("second not queued")
	}
	pool.Forget(second.ID)
	close(block)
	if pool.Put(context.Background(), third, trace.Link{}) != queued {
		t.Fatal("third not queued")
	}
	for {
		id := <-push.finished
		if id == third.ID {
			break
		}
	}
	if push.has(second.ID) {
		t.Fatal("cancelled trip was sent")
	}
	if !push.has(first.ID) || !push.has(third.ID) {
		t.Fatal("expected the other trips to be sent")
	}
}

func TestFullQueueDropsWithoutBlocking(t *testing.T) {
	block := make(chan struct{})
	push := newGate(block)
	checks := &memCount{}
	requests := &memCount{}
	ticks := make(chan time.Time)
	pool := newPool(t, newSrc(nil), push, checks, requests, 1, 1, ticks)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(root)
	defer func() { _ = pool.Stop(context.Background()) }()

	if pool.Put(context.Background(), quietTrip(), trace.Link{}) != queued {
		t.Fatal("worker job")
	}
	<-push.entered
	if pool.Put(context.Background(), quietTrip(), trace.Link{}) != queued {
		t.Fatal("queue slot")
	}
	if got := pool.Put(context.Background(), quietTrip(), trace.Link{}); got != "dropped" {
		t.Fatalf("put %s", got)
	}
	if checks.get("dropped") != 1 || requests.get("dropped") != 1 {
		t.Fatalf("checks %d requests %d", checks.get("dropped"), requests.get("dropped"))
	}
	close(block)
}

func TestWorkersStayWithinLimit(t *testing.T) {
	const workers = 3
	const jobs = 7
	release := make(chan struct{})
	var current atomic.Int32
	var maxSeen atomic.Int32
	push := newGate(release)
	push.onEnter = func() {
		n := current.Add(1)
		for {
			old := maxSeen.Load()
			if n <= old || maxSeen.CompareAndSwap(old, n) {
				break
			}
		}
	}
	push.onLeave = func() { current.Add(-1) }
	ticks := make(chan time.Time)
	pool := newPool(t, newSrc(nil), push, &memCount{}, &memCount{}, workers, jobs, ticks)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(root)
	defer func() { _ = pool.Stop(context.Background()) }()

	for range jobs {
		if pool.Put(context.Background(), quietTrip(), trace.Link{}) != queued {
			t.Fatal("not queued")
		}
	}
	for range workers {
		<-push.entered
	}
	if maxSeen.Load() != int32(workers) {
		t.Fatalf("max %d", maxSeen.Load())
	}
	close(release)
	for range jobs {
		<-push.finished
	}
	if maxSeen.Load() != int32(workers) {
		t.Fatalf("max after %d", maxSeen.Load())
	}
}

func TestPushErrorDoesNotStopWorker(t *testing.T) {
	bad := quietTrip()
	good := quietTrip()
	push := newGate(nil)
	push.fail[bad.ID] = true
	requests := &memCount{}
	ticks := make(chan time.Time)
	pool := newPool(t, newSrc(nil), push, &memCount{}, requests, 1, 4, ticks)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(root)
	defer func() { _ = pool.Stop(context.Background()) }()

	if pool.Put(context.Background(), bad, trace.Link{}) != queued {
		t.Fatal("bad")
	}
	if pool.Put(context.Background(), good, trace.Link{}) != queued {
		t.Fatal("good")
	}
	<-push.finished
	<-push.finished
	if requests.get("error") != 1 || requests.get("success") != 1 {
		t.Fatalf("error %d success %d", requests.get("error"), requests.get("success"))
	}
}

func TestStopLeavesNoGoroutines(t *testing.T) {
	ticks := make(chan time.Time)
	pool := newPool(t, newSrc(nil), newGate(nil), &memCount{}, &memCount{}, 2, 2, ticks)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(root)
	if err := pool.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestScanTicksDoNotOverlap(t *testing.T) {
	hold := make(chan struct{})
	src := newSrc(nil)
	src.hold = hold
	ticks := make(chan time.Time)
	pool := newPool(t, src, newGate(nil), &memCount{}, &memCount{}, 1, 2, ticks)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(root)
	defer func() { _ = pool.Stop(context.Background()) }()

	ticks <- time.Now()
	<-src.entered
	select {
	case ticks <- time.Now():
		t.Fatal("second tick was consumed during the first scan")
	default:
	}
	if src.calls.Load() != 1 {
		t.Fatalf("calls %d", src.calls.Load())
	}
	close(hold)
}

func newPool(t *testing.T, src *quietSrc, push *gate, checks, requests *memCount, workers, queue int, ticks chan time.Time) *positions.Pool {
	t.Helper()
	return positions.New(
		slog.New(slog.DiscardHandler),
		src,
		push,
		positions.Config{
			Interval:    time.Hour,
			Stale:       time.Minute,
			Workers:     workers,
			Queue:       queue,
			PushTimeout: time.Second,
			Clock: func(time.Duration) (<-chan time.Time, func()) {
				return ticks, func() {}
			},
		},
		checks,
		requests,
		memTime{},
	)
}

func quietTrip() domaintrip.Quiet {
	return domaintrip.Quiet{ID: uuid.NewString(), DriverID: uuid.NewString(), Since: time.Now().Add(-time.Hour)}
}

type memCount struct {
	mu sync.Mutex
	n  map[string]int
}

func (m *memCount) Add(_ context.Context, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.n == nil {
		m.n = map[string]int{}
	}
	m.n[result]++
}

func (m *memCount) get(result string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.n[result]
}

type memTime struct{}

func (memTime) Record(context.Context, float64, string) {}

type quietSrc struct {
	mu      sync.Mutex
	rows    []domaintrip.Quiet
	calls   atomic.Int32
	hold    chan struct{}
	entered chan struct{}
	called  chan struct{}
}

func newSrc(rows []domaintrip.Quiet) *quietSrc {
	return &quietSrc{
		rows:    rows,
		entered: make(chan struct{}, 1),
		called:  make(chan struct{}, 4),
	}
}

func (s *quietSrc) Quiet(ctx context.Context, _ time.Time) ([]domaintrip.Quiet, error) {
	n := s.calls.Add(1)
	s.mu.Lock()
	rows := append([]domaintrip.Quiet(nil), s.rows...)
	s.mu.Unlock()
	s.called <- struct{}{}
	if n == 1 && s.hold != nil {
		s.entered <- struct{}{}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.hold:
		}
	}
	return rows, nil
}

type gate struct {
	mu       sync.Mutex
	ids      []string
	fail     map[string]bool
	block    chan struct{}
	entered  chan struct{}
	finished chan string
	onEnter  func()
	onLeave  func()
}

func newGate(block chan struct{}) *gate {
	return &gate{
		fail:     map[string]bool{},
		block:    block,
		entered:  make(chan struct{}, 8),
		finished: make(chan string, 8),
	}
}

func (g *gate) RequestPosition(ctx context.Context, _, tripID string) error {
	if g.onEnter != nil {
		g.onEnter()
	}
	g.entered <- struct{}{}
	if g.block != nil {
		select {
		case <-ctx.Done():
			if g.onLeave != nil {
				g.onLeave()
			}
			return ctx.Err()
		case <-g.block:
		}
	}
	if g.onLeave != nil {
		g.onLeave()
	}
	g.mu.Lock()
	g.ids = append(g.ids, tripID)
	g.mu.Unlock()
	g.finished <- tripID
	if g.fail[tripID] {
		return errPush
	}
	return nil
}

func (g *gate) has(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, item := range g.ids {
		if item == id {
			return true
		}
	}
	return false
}

type pushError string

func (e pushError) Error() string { return string(e) }

const errPush pushError = "push недоступен"

package finish_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/usecase/trip/finish"
)

func TestFinish(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	tx := NewMockTxManager(ctrl)
	counter := NewMockCounter(ctrl)
	uc := finish.New(repo, tx, slog.New(slog.DiscardHandler), counter)

	item := activeTrip(t)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runTx)
	repo.EXPECT().Find(gomock.Any(), item.ID).Return(item, nil)
	repo.EXPECT().Complete(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got trip.Trip) error {
		if got.Status != trip.StatusCompleted || got.FinishedAt == nil {
			t.Fatalf("complete %+v", got)
		}
		return nil
	})
	counter.EXPECT().Add(gomock.Any(), int64(1))

	got, err := uc.Finish(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != trip.StatusCompleted {
		t.Fatalf("status %s", got.Status)
	}
}

func TestFinishCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	tx := NewMockTxManager(ctrl)
	uc := finish.New(repo, tx, slog.New(slog.DiscardHandler), NewMockCounter(ctrl))

	item := activeTrip(t)
	if err := item.Finish(time.Now()); err != nil {
		t.Fatal(err)
	}
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runTx)
	repo.EXPECT().Find(gomock.Any(), item.ID).Return(item, nil)

	_, err := uc.Finish(context.Background(), item.ID)
	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Fatalf("err %v", err)
	}
}

func TestFinishMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	tx := NewMockTxManager(ctrl)
	uc := finish.New(repo, tx, slog.New(slog.DiscardHandler), NewMockCounter(ctrl))

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runTx)
	repo.EXPECT().Find(gomock.Any(), "missing").Return(trip.Trip{}, trip.ErrTripNotFound)

	_, err := uc.Finish(context.Background(), "missing")
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("err %v", err)
	}
}

func activeTrip(t *testing.T) trip.Trip {
	t.Helper()
	item, err := trip.New(uuid.NewString(), uuid.NewString(), uuid.NewString(), 10, 20, 30, 40, 100, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func runTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

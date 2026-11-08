package create_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/model"
	"github.com/sonjiwu2/tripgo/internal/usecase/trip/create"
)

func TestCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	tx := NewMockTxManager(ctrl)
	counter := NewMockCounter(ctrl)
	uc := create.New(repo, tx, slog.New(slog.DiscardHandler), counter)
	draft := sampleDraft()

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runTx)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, item trip.Trip) error {
		if item.UserID != draft.UserID || item.DriverID != draft.DriverID || item.Price != draft.Price {
			t.Fatalf("saved %+v", item)
		}
		if item.StartLatitude != draft.StartLatitude || item.EndLatitude != draft.EndLatitude {
			t.Fatalf("latitude %+v", item)
		}
		if item.StartLongitude != draft.StartLongitude || item.EndLongitude != draft.EndLongitude {
			t.Fatalf("longitude %+v", item)
		}
		if item.Status != trip.StatusActive {
			t.Fatalf("status %s", item.Status)
		}
		return nil
	})
	counter.EXPECT().Add(gomock.Any(), int64(1))

	got, err := uc.Create(context.Background(), draft)
	if err != nil {
		t.Fatal(err)
	}
	if got.DriverID != draft.DriverID || got.Status != trip.StatusActive {
		t.Fatalf("got %+v", got)
	}
}

func TestCreateDriverBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	tx := NewMockTxManager(ctrl)
	uc := create.New(repo, tx, slog.New(slog.DiscardHandler), NewMockCounter(ctrl))

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runTx)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(trip.ErrDriverBusy)

	_, err := uc.Create(context.Background(), sampleDraft())
	if !errors.Is(err, trip.ErrDriverBusy) {
		t.Fatalf("err %v", err)
	}
}

func TestCreateRejectsBadDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := create.New(NewMockTripRepository(ctrl), NewMockTxManager(ctrl), slog.New(slog.DiscardHandler), NewMockCounter(ctrl))
	draft := sampleDraft()
	draft.Price = -5

	_, err := uc.Create(context.Background(), draft)
	var field *trip.FieldError
	if !errors.As(err, &field) {
		t.Fatalf("err %v", err)
	}
	if field.Field != "price" {
		t.Fatalf("field %s", field.Field)
	}
}

func sampleDraft() model.Draft {
	return model.Draft{
		UserID:         uuid.NewString(),
		DriverID:       uuid.NewString(),
		StartLatitude:  59.93,
		StartLongitude: 30.31,
		EndLatitude:    59.95,
		EndLongitude:   30.35,
		Price:          450,
	}
}

func runTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

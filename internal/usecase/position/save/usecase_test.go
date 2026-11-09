package save_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/usecase/position/save"
)

func TestSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockPositionStore(ctrl)
	tx := NewMockTxManager(ctrl)
	counter := NewMockCounter(ctrl)
	uc := save.New(repo, tx, counter)
	tripID := uuid.NewString()
	recorded := time.Now().Add(-time.Minute)

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runTx)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, point trip.Point) (trip.Point, error) {
		if point.TripID != tripID || point.Latitude != 59.93 || point.Longitude != 30.33 {
			t.Fatalf("point %+v", point)
		}
		point.ID = 7
		return point, nil
	})
	counter.EXPECT().Add(gomock.Any(), int64(1))

	got, err := uc.Save(context.Background(), tripID, 59.93, 30.33, recorded)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.TripID != tripID {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveKeepsDomainError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockPositionStore(ctrl)
	tx := NewMockTxManager(ctrl)
	uc := save.New(repo, tx, NewMockCounter(ctrl))

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runTx)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(trip.Point{}, trip.ErrTripCompleted)

	_, err := uc.Save(context.Background(), uuid.NewString(), 1, 1, time.Now())
	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Fatalf("err %v", err)
	}
}

func TestSaveRejectsFuturePoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := save.New(NewMockPositionStore(ctrl), NewMockTxManager(ctrl), NewMockCounter(ctrl))

	_, err := uc.Save(context.Background(), uuid.NewString(), 1, 1, time.Now().Add(2*time.Minute))
	var field *trip.FieldError
	if !errors.As(err, &field) {
		t.Fatalf("err %v", err)
	}
	if field.Field != "recorded_at" {
		t.Fatalf("field %s", field.Field)
	}
}

func runTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

package get_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/usecase/trip/get"
)

func TestGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	uc := get.New(repo)

	item, err := trip.New(uuid.NewString(), uuid.NewString(), uuid.NewString(), 1, 2, 3, 4, 10, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	repo.EXPECT().Find(gomock.Any(), item.ID).Return(item, nil)

	got, err := uc.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != item.ID {
		t.Fatalf("id %s", got.ID)
	}
}

func TestGetMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	uc := get.New(repo)
	repo.EXPECT().Find(gomock.Any(), "missing").Return(trip.Trip{}, trip.ErrTripNotFound)

	_, err := uc.Get(context.Background(), "missing")
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("err %v", err)
	}
}

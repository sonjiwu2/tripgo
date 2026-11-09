package list_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/usecase/position/list"
)

func TestListMissingTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	trips := NewMockTripStore(ctrl)
	uc := list.New(NewMockPositionStore(ctrl), trips)
	id := uuid.NewString()

	trips.EXPECT().Find(gomock.Any(), id).Return(trip.Trip{}, trip.ErrTripNotFound)

	_, err := uc.List(context.Background(), id)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("err %v", err)
	}
}

func TestList(t *testing.T) {
	ctrl := gomock.NewController(t)
	trips := NewMockTripStore(ctrl)
	points := NewMockPositionStore(ctrl)
	uc := list.New(points, trips)
	id := uuid.NewString()
	when := time.Now().Add(-time.Minute)

	trips.EXPECT().Find(gomock.Any(), id).Return(trip.Trip{ID: id}, nil)
	points.EXPECT().List(gomock.Any(), id).Return([]trip.Point{{
		ID: idNum(t), TripID: id, Latitude: 1, Longitude: 2, RecordedAt: when,
	}}, nil)

	got, err := uc.List(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TripID != id || !got[0].RecordedAt.Equal(when) {
		t.Fatalf("got %+v", got)
	}
}

func idNum(t *testing.T) int64 {
	t.Helper()
	return 4
}

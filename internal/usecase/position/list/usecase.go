package list

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

type Usecase struct {
	points positionStore
	trips  tripStore
}

func New(points positionStore, trips tripStore) *Usecase {
	return &Usecase{points: points, trips: trips}
}

func (u *Usecase) List(ctx context.Context, tripID string) ([]domaintrip.Point, error) {
	if _, err := u.trips.Find(ctx, tripID); err != nil {
		return nil, err
	}
	return u.points.List(ctx, tripID)
}

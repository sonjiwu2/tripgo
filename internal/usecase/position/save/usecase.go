package save

import (
	"context"
	"time"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

type Usecase struct {
	repo  positionStore
	tx    txManager
	saved counter
}

func New(repo positionStore, tx txManager, saved counter) *Usecase {
	return &Usecase{repo: repo, tx: tx, saved: saved}
}

func (u *Usecase) Save(ctx context.Context, tripID string, lat, lon float64, recorded time.Time) (domaintrip.Point, error) {
	point, err := domaintrip.NewPoint(tripID, lat, lon, recorded, time.Now())
	if err != nil {
		return domaintrip.Point{}, err
	}

	var stored domaintrip.Point
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		var saveErr error
		stored, saveErr = u.repo.Save(ctx, point)
		return saveErr
	})
	if err != nil {
		return domaintrip.Point{}, err
	}
	u.saved.Add(ctx, 1)
	return stored, nil
}

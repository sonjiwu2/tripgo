package create

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/model"
)

type Usecase struct {
	repo    tripRepository
	tx      txManager
	log     *slog.Logger
	created counter
}

func New(repo tripRepository, tx txManager, log *slog.Logger, created counter) *Usecase {
	return &Usecase{repo: repo, tx: tx, log: log, created: created}
}

func (u *Usecase) Create(ctx context.Context, in model.Draft) (domaintrip.Trip, error) {
	item, err := domaintrip.New(
		uuid.NewString(),
		in.UserID,
		in.DriverID,
		in.StartLatitude,
		in.StartLongitude,
		in.EndLatitude,
		in.EndLongitude,
		in.Price,
		time.Now(),
	)
	if err != nil {
		return domaintrip.Trip{}, err
	}

	err = u.tx.Do(ctx, func(ctx context.Context) error {
		return u.repo.Save(ctx, item)
	})
	if err != nil {
		return domaintrip.Trip{}, err
	}

	u.created.Add(ctx, 1)
	u.log.InfoContext(ctx, "поездка создана", "trip_id", item.ID)
	return item, nil
}

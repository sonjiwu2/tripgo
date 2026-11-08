package finish

import (
	"context"
	"log/slog"
	"time"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

type Usecase struct {
	repo      tripRepository
	tx        txManager
	log       *slog.Logger
	completed counter
}

func New(repo tripRepository, tx txManager, log *slog.Logger, completed counter) *Usecase {
	return &Usecase{repo: repo, tx: tx, log: log, completed: completed}
}

func (u *Usecase) Finish(ctx context.Context, id string) (domaintrip.Trip, error) {
	var done domaintrip.Trip
	err := u.tx.Do(ctx, func(ctx context.Context) error {
		found, err := u.repo.Find(ctx, id)
		if err != nil {
			return err
		}
		if err = found.Finish(time.Now()); err != nil {
			return err
		}
		if err = u.repo.Complete(ctx, found); err != nil {
			return err
		}
		done = found
		return nil
	})
	if err != nil {
		return domaintrip.Trip{}, err
	}

	u.completed.Add(ctx, 1)
	u.log.InfoContext(ctx, "поездка завершена", "trip_id", done.ID)
	return done, nil
}

package get

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

type Usecase struct {
	repo tripRepository
}

func New(repo tripRepository) *Usecase {
	return &Usecase{repo: repo}
}

func (u *Usecase) Get(ctx context.Context, id string) (domaintrip.Trip, error) {
	found, err := u.repo.Find(ctx, id)
	if err != nil {
		return domaintrip.Trip{}, err
	}
	return found, nil
}

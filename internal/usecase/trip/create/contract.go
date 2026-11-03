package create

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate mockgen -source contract.go -destination mocks_test.go -package create_test

type (
	tripRepository interface {
		Save(ctx context.Context, item domaintrip.Trip) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	counter interface {
		Add(ctx context.Context, value int64)
	}
)

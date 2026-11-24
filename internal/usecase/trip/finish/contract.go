package finish

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package finish_test -mock_names tripRepository=MockTripRepository,txManager=MockTxManager,counter=MockCounter

type (
	tripRepository interface {
		Find(ctx context.Context, id string) (domaintrip.Trip, error)
		Complete(ctx context.Context, item domaintrip.Trip) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	counter interface {
		Add(ctx context.Context, value int64)
	}
)

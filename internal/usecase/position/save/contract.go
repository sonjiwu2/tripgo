package save

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package save_test -mock_names positionStore=MockPositionStore,txManager=MockTxManager,counter=MockCounter

type (
	positionStore interface {
		Save(ctx context.Context, point domaintrip.Point) (domaintrip.Point, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	counter interface {
		Add(ctx context.Context, value int64)
	}
)

package get

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package get_test -mock_names tripReader=MockTripReader

type tripReader interface {
	Get(ctx context.Context, id string) (domaintrip.Trip, error)
}

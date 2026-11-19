package list

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package list_test -mock_names routeReader=MockRouteReader

type routeReader interface {
	List(ctx context.Context, tripID string) ([]domaintrip.Point, error)
}

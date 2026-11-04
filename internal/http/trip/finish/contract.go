package finish

import (
	"context"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package finish_test -mock_names tripFinisher=MockTripFinisher

type tripFinisher interface {
	Finish(ctx context.Context, id string) (domaintrip.Trip, error)
}

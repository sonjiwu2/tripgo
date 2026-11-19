package save

import (
	"context"
	"time"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package save_test -mock_names pointSaver=MockPointSaver,waitingTrips=MockWaitingTrips

type (
	pointSaver interface {
		Save(ctx context.Context, tripID string, lat, lon float64, recorded time.Time) (domaintrip.Point, error)
	}

	waitingTrips interface {
		Forget(tripID string)
	}
)

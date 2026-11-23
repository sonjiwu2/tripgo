package positions

import (
	"context"
	"time"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package positions_test -mock_names source=MockSource,pusher=MockPusher,resultCounter=MockResultCounter,durationRecorder=MockDurationRecorder,rowsRecorder=MockRowsRecorder

type (
	source interface {
		Quiet(ctx context.Context, at time.Time, staleAfter time.Duration, after domaintrip.Quiet, limit int) ([]domaintrip.Quiet, error)
	}

	pusher interface {
		RequestPosition(ctx context.Context, driverID, tripID string) error
	}

	resultCounter interface {
		Add(ctx context.Context, result string)
	}

	durationRecorder interface {
		Record(ctx context.Context, seconds float64, result string)
	}

	rowsRecorder interface {
		Record(ctx context.Context, rows int64)
	}
)

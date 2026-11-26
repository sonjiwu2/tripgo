package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Meters struct {
	HTTPDuration     metric.Float64Histogram
	HTTPActive       metric.Int64UpDownCounter
	Created          Counter
	Completed        Counter
	PositionsSaved   Counter
	StaleChecks      ResultCounter
	PositionRequests ResultCounter
	PositionDuration Duration
	RowsRead         Rows
}

type Counter struct {
	counter metric.Int64Counter
}

func (c Counter) Add(ctx context.Context, value int64) {
	c.counter.Add(ctx, value)
}

type ResultCounter struct {
	counter metric.Int64Counter
}

func (c ResultCounter) Add(ctx context.Context, result string) {
	c.counter.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

type Duration struct {
	hist metric.Float64Histogram
}

func (d Duration) Record(ctx context.Context, seconds float64, result string) {
	d.hist.Record(ctx, seconds, metric.WithAttributes(attribute.String("result", result)))
}

type Rows struct {
	hist metric.Int64Histogram
}

func (r Rows) Record(ctx context.Context, rows int64) {
	r.hist.Record(ctx, rows)
}

type activeTrips interface {
	CountActive(ctx context.Context) (int64, error)
}

func NewMeters() (Meters, error) {
	meter := otel.Meter("github.com/sonjiwu2/tripgo")
	duration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	if err != nil {
		return Meters{}, err
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return Meters{}, err
	}
	created, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return Meters{}, err
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return Meters{}, err
	}
	saved, err := meter.Int64Counter("tripgo.positions.saved", metric.WithUnit("{position}"))
	if err != nil {
		return Meters{}, err
	}
	checks, err := meter.Int64Counter("tripgo.stale_checks", metric.WithUnit("{check}"))
	if err != nil {
		return Meters{}, err
	}
	requests, err := meter.Int64Counter("tripgo.position_requests", metric.WithUnit("{request}"))
	if err != nil {
		return Meters{}, err
	}
	pushDuration, err := meter.Float64Histogram(
		"tripgo.position_requests.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5),
	)
	if err != nil {
		return Meters{}, err
	}
	rowsRead, err := meter.Int64Histogram("tripgo.stale_checks.rows_read", metric.WithUnit("{row}"))
	if err != nil {
		return Meters{}, err
	}
	return Meters{
		RowsRead:         Rows{hist: rowsRead},
		HTTPDuration:     duration,
		HTTPActive:       active,
		Created:          Counter{counter: created},
		Completed:        Counter{counter: completed},
		PositionsSaved:   Counter{counter: saved},
		StaleChecks:      ResultCounter{counter: checks},
		PositionRequests: ResultCounter{counter: requests},
		PositionDuration: Duration{hist: pushDuration},
	}, nil
}

func WatchActive(log *slog.Logger, src activeTrips) error {
	meter := otel.Meter("github.com/sonjiwu2/tripgo")
	gauge, err := meter.Int64ObservableGauge("tripgo.trips.active", metric.WithUnit("{trip}"))
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		count, err := src.CountActive(ctx)
		if err != nil {
			log.ErrorContext(ctx, "не получилось посчитать активные поездки", "err", err)
			return nil
		}
		observer.ObserveInt64(gauge, count)
		return nil
	}, gauge)
	return err
}

type positionLoad interface {
	QueueSize() int64
	WorkersBusy() int64
}

func WatchPositions(src positionLoad) error {
	meter := otel.Meter("github.com/sonjiwu2/tripgo")
	queue, err := meter.Int64ObservableGauge("tripgo.position_requests.queue.size", metric.WithUnit("{task}"))
	if err != nil {
		return err
	}
	busy, err := meter.Int64ObservableGauge("tripgo.position_requests.workers.active", metric.WithUnit("{worker}"))
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(queue, src.QueueSize())
		observer.ObserveInt64(busy, src.WorkersBusy())
		return nil
	}, queue, busy)
	return err
}

package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type Meters struct {
	HTTPDuration metric.Float64Histogram
	HTTPActive   metric.Int64UpDownCounter
	Created      Counter
	Completed    Counter
}

type Counter struct {
	counter metric.Int64Counter
}

func (c Counter) Add(ctx context.Context, value int64) {
	c.counter.Add(ctx, value)
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
	return Meters{
		HTTPDuration: duration,
		HTTPActive:   active,
		Created:      Counter{counter: created},
		Completed:    Counter{counter: completed},
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

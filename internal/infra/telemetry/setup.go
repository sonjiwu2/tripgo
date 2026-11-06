package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/sonjiwu2/tripgo/internal/config"
)

const serviceVersion = "dev"

func Setup(ctx context.Context, cfg config.OTEL) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(attribute.String("service.version", serviceVersion)),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := newTraceExporter(ctx, cfg.Protocol)
	if err != nil {
		return nil, err
	}
	metricExporter, err := newMetricExporter(ctx, cfg.Protocol)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func(ctx context.Context) error {
		return errors.Join(meterProvider.Shutdown(ctx), tracerProvider.Shutdown(ctx))
	}, nil
}

func newTraceExporter(ctx context.Context, protocol string) (sdktrace.SpanExporter, error) {
	if protocol == "grpc" {
		return otlptracegrpc.New(ctx)
	}
	return otlptracehttp.New(ctx)
}

func newMetricExporter(ctx context.Context, protocol string) (sdkmetric.Exporter, error) {
	if protocol == "grpc" {
		return otlpmetricgrpc.New(ctx)
	}
	return otlpmetrichttp.New(ctx)
}

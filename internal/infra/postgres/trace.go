package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type queryTracer struct {
	spans    *otelpgx.Tracer
	duration metric.Float64Histogram
}

type queryInfo struct {
	operation string
	table     string
	start     time.Time
}

type queryInfoKey struct{}

func newQueryTracer() (*queryTracer, error) {
	duration, err := otel.Meter("github.com/sonjiwu2/tripgo").Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10),
	)
	if err != nil {
		return nil, err
	}
	spans := otelpgx.NewTracer(
		otelpgx.WithMeterProvider(noopmetric.NewMeterProvider()),
		otelpgx.WithTrimSQLInSpanName(),
	)
	return &queryTracer{spans: spans, duration: duration}, nil
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	operation, table := parseQuery(data.SQL)
	ctx = t.spans.TraceQueryStart(ctx, conn, data)
	trace.SpanFromContext(ctx).SetAttributes(
		attribute.String("db.operation.name", operation),
		attribute.String("db.collection.name", table),
	)
	return context.WithValue(ctx, queryInfoKey{}, queryInfo{operation: operation, table: table, start: time.Now()})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	info, _ := ctx.Value(queryInfoKey{}).(queryInfo)
	attrs := []attribute.KeyValue{
		attribute.String("db.operation.name", info.operation),
		attribute.String("db.collection.name", info.table),
	}
	if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		errAttr := attribute.String("error.type", errorType(data.Err))
		attrs = append(attrs, errAttr)
		trace.SpanFromContext(ctx).SetAttributes(errAttr)
	}
	t.duration.Record(ctx, time.Since(info.start).Seconds(), metric.WithAttributes(attrs...))
	t.spans.TraceQueryEnd(ctx, conn, data)
}

func errorType(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return "query_failed"
}

func parseQuery(sql string) (operation, table string) {
	words := strings.Fields(sql)
	if len(words) == 0 {
		return "QUERY", "unknown"
	}
	for i, word := range words[:len(words)-1] {
		switch strings.ToUpper(word) {
		case "FROM", "INTO", "UPDATE":
			return strings.ToUpper(words[0]), words[i+1]
		}
	}
	return strings.ToUpper(words[0]), "unknown"
}

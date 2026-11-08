package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestLoggerAddsTraceIDs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(traceHandler{next: slog.NewJSONHandler(&buf, nil)})
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	ctx, span := provider.Tracer("test").Start(context.Background(), "request")
	log.InfoContext(ctx, "request handled", "route", "/api/v1/trips")
	span.End()

	text := buf.String()
	if !strings.Contains(text, `"trace_id":"`+span.SpanContext().TraceID().String()+`"`) {
		t.Fatalf("log %s", text)
	}
	if !strings.Contains(text, `"span_id":"`+span.SpanContext().SpanID().String()+`"`) {
		t.Fatalf("log %s", text)
	}
}

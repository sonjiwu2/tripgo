package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestExportersUseSignalPaths(t *testing.T) {
	paths := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)

	ctx := context.Background()

	traces, err := newTraceExporter(ctx, "http/protobuf")
	if err != nil {
		t.Fatal(err)
	}
	spans := tracetest.SpanStubs{{Name: "request"}}.Snapshots()
	if err = traces.ExportSpans(ctx, spans); err != nil {
		t.Fatal(err)
	}
	if got := <-paths; got != "/v1/traces" {
		t.Fatalf("traces path %q", got)
	}

	metrics, err := newMetricExporter(ctx, "http/protobuf")
	if err != nil {
		t.Fatal(err)
	}
	if err = metrics.Export(ctx, &metricdata.ResourceMetrics{}); err != nil {
		t.Fatal(err)
	}
	if got := <-paths; got != "/v1/metrics" {
		t.Fatalf("metrics path %q", got)
	}
}

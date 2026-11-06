package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics(mux *chi.Mux, duration metric.Float64Histogram, active metric.Int64UpDownCounter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := matchPattern(mux, r)
			labels := []attribute.KeyValue{
				attribute.String(attrMethod, r.Method),
				attribute.String(attrRoute, pattern),
			}
			active.Add(r.Context(), 1, metric.WithAttributes(labels...))
			defer active.Add(r.Context(), -1, metric.WithAttributes(labels...))

			started := time.Now()
			rec := newCapture(w)
			next.ServeHTTP(rec, r)
			duration.Record(r.Context(), time.Since(started).Seconds(), metric.WithAttributes(
				attribute.String(attrMethod, r.Method),
				attribute.String(attrRoute, pattern),
				attribute.Int(attrStatus, rec.status),
			))
		})
	}
}

func matchPattern(mux *chi.Mux, r *http.Request) string {
	found := mux.Find(chi.NewRouteContext(), r.Method, r.URL.Path)
	if found != "" {
		return found
	}
	return r.URL.Path
}

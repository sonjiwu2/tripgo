package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

const (
	attrMethod  = "http.request.method"
	attrRoute   = "http.route"
	attrStatus  = "http.response.status_code"
	attrErr     = "error.type"
	errInternal = "internal_error"
)

func Tracing() func(http.Handler) http.Handler {
	wrapped := otelhttp.NewMiddleware(
		"http",
		otelhttp.WithMeterProvider(noopmetric.NewMeterProvider()),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + Pattern(r)
		}),
	)
	return func(next http.Handler) http.Handler {
		return wrapped(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := newCapture(w)
			defer func() {
				span := trace.SpanFromContext(r.Context())
				pattern := Pattern(r)
				span.SetName(r.Method + " " + pattern)
				span.SetAttributes(
					attribute.String(attrMethod, r.Method),
					attribute.String(attrRoute, pattern),
					attribute.Int(attrStatus, rec.status),
				)
				if rec.status >= http.StatusInternalServerError {
					span.SetStatus(codes.Error, errInternal)
					span.SetAttributes(attribute.String(attrErr, errInternal))
				}
				if got := recover(); got != nil {
					span.SetStatus(codes.Error, errInternal)
					span.SetAttributes(attribute.String(attrErr, errInternal))
					panic(got)
				}
			}()
			next.ServeHTTP(rec, r)
		}))
	}
}

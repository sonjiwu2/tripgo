package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestPatternUsesTemplate(t *testing.T) {
	var got string
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			got = Pattern(r)
		})
	})
	router.Get("/api/v1/trips/{tripId}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got != "/api/v1/trips/{tripId}" {
		t.Fatalf("route %q", got)
	}
	if strings.Contains(got, id) {
		t.Fatalf("id leaked into route %q", got)
	}
}

func TestTracingUsesRouteTemplate(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	router := chi.NewRouter()
	router.Use(Tracing())
	router.Post("/api/v1/trips/{tripId}/finish", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id+"/finish", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans %d", len(spans))
	}
	if spans[0].Name() != "POST /api/v1/trips/{tripId}/finish" {
		t.Fatalf("name %s", spans[0].Name())
	}
	if strings.Contains(spans[0].Name(), id) {
		t.Fatalf("id leaked: %s", spans[0].Name())
	}
	if spans[0].Status().Code != codes.Error {
		t.Fatalf("status %s", spans[0].Status().Code)
	}
	var errType string
	for _, attr := range spans[0].Attributes() {
		if string(attr.Key) == attrErr {
			errType = attr.Value.AsString()
		}
	}
	if errType != errInternal {
		t.Fatalf("error.type %s", errType)
	}
}

func TestMetricsUsesRouteTemplate(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/api/v1/trips/{tripId}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id, nil)
	got := matchPattern(router, req)
	if got != "/api/v1/trips/{tripId}" {
		t.Fatalf("route %s", got)
	}
}

func TestRecover(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	handler := Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret-panic-text")
	}))

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "secret-panic-text") {
			t.Fatalf("panic leaked: %s", body)
		}
		if !strings.Contains(body, "internal_error") {
			t.Fatalf("body %s", body)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatalf("content-type %s", rec.Header().Get("Content-Type"))
		}
	}
}

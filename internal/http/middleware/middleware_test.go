package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

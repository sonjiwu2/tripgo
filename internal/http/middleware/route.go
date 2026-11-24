package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Pattern(r *http.Request) string {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return r.URL.Path
	}
	if rc.RoutePattern() != "" {
		return rc.RoutePattern()
	}
	return r.URL.Path
}

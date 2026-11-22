package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const unknownRoute = "unknown"

func Pattern(r *http.Request) string {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return unknownRoute
	}
	if rc.RoutePattern() != "" {
		return rc.RoutePattern()
	}
	return unknownRoute
}

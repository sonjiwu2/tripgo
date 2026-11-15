package probe

import (
	"context"
	"net/http"
	"time"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/http/respond"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	db      pinger
	timeout time.Duration
}

func New(db pinger, timeout time.Duration) *Handler {
	return &Handler{db: db, timeout: timeout}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		respond.JSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	respond.JSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

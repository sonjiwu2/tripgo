package list

import (
	"log/slog"
	"net/http"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/http/domainerror"
	"github.com/sonjiwu2/tripgo/internal/http/present"
	"github.com/sonjiwu2/tripgo/internal/http/respond"
)

type Handler struct {
	routes routeReader
	log    *slog.Logger
}

func NewHandler(routes routeReader, log *slog.Logger) *Handler {
	return &Handler{routes: routes, log: log}
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	items, err := h.routes.List(r.Context(), tripID.String())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.TripPosition, 0, len(items))
	for _, item := range items {
		out = append(out, present.Point(item))
	}
	respond.JSON(w, http.StatusOK, api.TripPositionsResponse{Positions: out})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.Problem(err)
	if status == http.StatusInternalServerError {
		h.log.ErrorContext(r.Context(), "запрос не обработан", "err", err)
	}
	path := r.URL.Path
	problem.Instance = &path
	respond.Problem(w, status, problem)
}

package finish

import (
	"log/slog"
	"net/http"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/http/domainerror"
	"github.com/sonjiwu2/tripgo/internal/http/present"
	"github.com/sonjiwu2/tripgo/internal/http/respond"
)

type Handler struct {
	trips tripFinisher
	log   *slog.Logger
}

func NewHandler(trips tripFinisher, log *slog.Logger) *Handler {
	return &Handler{trips: trips, log: log}
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	done, err := h.trips.Finish(r.Context(), tripID.String())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, present.Trip(done))
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

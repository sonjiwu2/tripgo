package create

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/http/domainerror"
	"github.com/sonjiwu2/tripgo/internal/http/present"
	"github.com/sonjiwu2/tripgo/internal/http/respond"
)

type Handler struct {
	trips tripCreator
	log   *slog.Logger
}

func NewHandler(trips tripCreator, log *slog.Logger) *Handler {
	return &Handler{trips: trips, log: log}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	var body api.TripData
	if !decode(w, r, &body) {
		return
	}

	made, err := h.trips.Create(r.Context(), draftFrom(body))
	if err != nil {
		h.fail(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+made.ID)
	respond.JSON(w, http.StatusCreated, present.Trip(made))
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

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeInvalid(w, r, "тело запроса не разобрать")
		return false
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		writeInvalid(w, r, "в теле лишние данные")
		return false
	}
	return true
}

func writeInvalid(w http.ResponseWriter, r *http.Request, detail string) {
	status, problem := domainerror.Invalid(detail)
	path := r.URL.Path
	problem.Instance = &path
	respond.Problem(w, status, problem)
}

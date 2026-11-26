package save

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/http/domainerror"
	"github.com/sonjiwu2/tripgo/internal/http/present"
	"github.com/sonjiwu2/tripgo/internal/http/respond"
)

type positionBody struct {
	Coordinates *coordinatesBody `json:"coordinates"`
	RecordedAt  *time.Time       `json:"recorded_at"`
}

type coordinatesBody struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

func (b positionBody) complete() bool {
	return b.RecordedAt != nil && b.Coordinates.complete()
}

func (c *coordinatesBody) complete() bool {
	return c != nil && c.Latitude != nil && c.Longitude != nil
}

type Handler struct {
	points  pointSaver
	waiting waitingTrips
	log     *slog.Logger
}

func NewHandler(points pointSaver, waiting waitingTrips, log *slog.Logger) *Handler {
	return &Handler{points: points, waiting: waiting, log: log}
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	var body positionBody
	if !decode(w, r, &body) {
		return
	}
	if !body.complete() {
		writeInvalid(w, r, "обязательные поля должны быть заполнены")
		return
	}

	saved, err := h.points.Save(
		r.Context(),
		tripID.String(),
		*body.Coordinates.Latitude,
		*body.Coordinates.Longitude,
		*body.RecordedAt,
	)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.waiting.Forget(r.Context(), tripID.String())
	h.log.InfoContext(r.Context(), "координата сохранена", "trip_id", tripID.String())
	respond.JSON(w, http.StatusCreated, present.Point(saved))
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

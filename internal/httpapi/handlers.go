package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/postgres"
	"github.com/sonjiwu2/tripgo/internal/trip"
)

type Handler struct {
	repo *trip.Repo
	tx   postgres.TxManager
	pool *pgxpool.Pool
}

func NewHandler(repo *trip.Repo, tx postgres.TxManager, pool *pgxpool.Pool) *Handler {
	return &Handler{repo: repo, tx: tx, pool: pool}
}

func Routes(h *Handler) http.Handler {
	router := chi.NewRouter()
	return api.HandlerWithOptions(h, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			fail(w, r, http.StatusBadRequest, "invalid_request", "tripId не UUID")
		},
	})
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	var body api.TripData
	if !decode(w, r, &body) {
		return
	}
	if !bodyOK(body) {
		fail(w, r, http.StatusBadRequest, "invalid_request", "проверь id, координаты и цену")
		return
	}

	fresh := trip.Trip{
		ID:             uuid.NewString(),
		UserID:         uuid.UUID(body.UserId).String(),
		DriverID:       uuid.UUID(body.DriverId).String(),
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
	}

	var created trip.Trip
	err := h.tx.Do(r.Context(), func(ctx context.Context) error {
		var err error
		created, err = h.repo.Create(ctx, fresh)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+created.ID)
	send(w, http.StatusCreated, view(created))
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	found, err := h.repo.Get(r.Context(), uuid.UUID(tripID).String())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	send(w, http.StatusOK, view(found))
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	var created trip.Trip
	err := h.tx.Do(r.Context(), func(ctx context.Context) error {
		var err error
		created, err = h.repo.Finish(ctx, uuid.UUID(tripID).String())
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	send(w, http.StatusOK, view(created))
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	send(w, http.StatusOK, api.HealthResponse{Status: "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.pool.Ping(r.Context()); err != nil {
		send(w, http.StatusServiceUnavailable, api.HealthResponse{Status: "unavailable"})
		return
	}
	send(w, http.StatusOK, api.HealthResponse{Status: "ok"})
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, _ api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, _ api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}

func bodyOK(body api.TripData) bool {
	if uuid.UUID(body.UserId) == uuid.Nil {
		return false
	}
	if uuid.UUID(body.DriverId) == uuid.Nil {
		return false
	}
	if body.Price < 0 {
		return false
	}
	if !coordsOK(body.StartPoint) {
		return false
	}
	return coordsOK(body.EndPoint)
}

func coordsOK(p api.Coordinates) bool {
	return p.Latitude >= -90 && p.Latitude <= 90 && p.Longitude >= -180 && p.Longitude <= 180
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		fail(w, r, http.StatusBadRequest, "invalid_request", "тело запроса не разобрать")
		return false
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		fail(w, r, http.StatusBadRequest, "invalid_request", "в теле лишние данные")
		return false
	}
	return true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, trip.ErrNotFound):
		fail(w, r, http.StatusNotFound, "trip_not_found", "такой поездки нет")
	case errors.Is(err, trip.ErrCompleted):
		fail(w, r, http.StatusConflict, "trip_completed", "эта поездка уже завершена")
	case errors.Is(err, trip.ErrDriverBusy):
		fail(w, r, http.StatusConflict, "driver_busy", "у водителя уже есть активная поездка")
	default:
		fail(w, r, http.StatusInternalServerError, "internal_error", "не получилось обработать запрос")
	}
}

func fail(w http.ResponseWriter, r *http.Request, status int, code, text string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	path := r.URL.Path
	_ = json.NewEncoder(w).Encode(api.Problem{
		Type:     "about:blank",
		Title:    text,
		Status:   int32(status),
		Code:     code,
		Detail:   &text,
		Instance: &path,
	})
}

func send(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func view(item trip.Trip) api.Trip {
	id, _ := uuid.Parse(item.ID)
	user, _ := uuid.Parse(item.UserID)
	driver, _ := uuid.Parse(item.DriverID)
	return api.Trip{
		Id:         openapi_types.UUID(id),
		UserId:     openapi_types.UUID(user),
		DriverId:   openapi_types.UUID(driver),
		StartPoint: api.Coordinates{Latitude: item.StartLatitude, Longitude: item.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: item.EndLatitude, Longitude: item.EndLongitude},
		Price:      item.Price,
		Status:     api.TripStatus(item.Status),
		StartedAt:  item.StartedAt,
		FinishedAt: item.FinishedAt,
	}
}

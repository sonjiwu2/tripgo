package router

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/metric"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/http/domainerror"
	"github.com/sonjiwu2/tripgo/internal/http/middleware"
	"github.com/sonjiwu2/tripgo/internal/http/probe"
	"github.com/sonjiwu2/tripgo/internal/http/respond"
	tripcreate "github.com/sonjiwu2/tripgo/internal/http/trip/create"
	tripfinish "github.com/sonjiwu2/tripgo/internal/http/trip/finish"
	tripget "github.com/sonjiwu2/tripgo/internal/http/trip/get"
)

type Handler struct {
	create *tripcreate.Handler
	get    *tripget.Handler
	finish *tripfinish.Handler
	probe  *probe.Handler
}

func New(create *tripcreate.Handler, get *tripget.Handler, finish *tripfinish.Handler, probe *probe.Handler) *Handler {
	return &Handler{create: create, get: get, finish: finish, probe: probe}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	h.create.CreateTrip(w, r, params)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.get.GetTrip(w, r, tripID)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.finish.FinishTrip(w, r, tripID)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.probe.Health(w, r)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	h.probe.Ready(w, r)
}

func Routes(h *Handler, log *slog.Logger, duration metric.Float64Histogram, active metric.Int64UpDownCounter) http.Handler {
	mux := chi.NewRouter()
	mux.Use(middleware.Recover(log))
	mux.Use(middleware.Tracing())
	mux.Use(middleware.Logging(log))
	mux.Use(middleware.Metrics(mux, duration, active))
	return api.HandlerWithOptions(h, api.ChiServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: badParam,
	})
}

func badParam(w http.ResponseWriter, r *http.Request, _ error) {
	status, problem := domainerror.Invalid("некорректный параметр запроса")
	path := r.URL.Path
	problem.Instance = &path
	respond.Problem(w, status, problem)
}

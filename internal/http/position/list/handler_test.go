package list_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/http/position/list"
)

func TestListTripPositionsEmpty(t *testing.T) {
	routes := NewMockRouteReader(gomock.NewController(t))
	h := list.NewHandler(routes, slog.New(slog.DiscardHandler))
	id := uuid.New()
	routes.EXPECT().List(gomock.Any(), id.String()).Return(nil, nil)

	rec := httptest.NewRecorder()
	h.ListTripPositions(rec, httptest.NewRequest(http.MethodGet, "/", nil), id)
	var body api.TripPositionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || body.Positions == nil || len(body.Positions) != 0 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestListTripPositionsNotFound(t *testing.T) {
	routes := NewMockRouteReader(gomock.NewController(t))
	h := list.NewHandler(routes, slog.New(slog.DiscardHandler))
	id := uuid.New()
	routes.EXPECT().List(gomock.Any(), id.String()).Return(nil, trip.ErrTripNotFound)

	rec := httptest.NewRecorder()
	h.ListTripPositions(rec, httptest.NewRequest(http.MethodGet, "/", nil), id)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "trip_not_found") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

package save_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/http/position/save"
)

func TestCreateTripPosition(t *testing.T) {
	ctrl := gomock.NewController(t)
	points := NewMockPointSaver(ctrl)
	waiting := NewMockWaitingTrips(ctrl)
	h := save.NewHandler(points, waiting, slog.New(slog.DiscardHandler))
	id := uuid.New()
	recorded := time.Now().Add(-time.Second).UTC().Format(time.RFC3339)

	points.EXPECT().Save(gomock.Any(), id.String(), 59.93, 30.33, gomock.Any()).
		Return(trip.Point{ID: 3, TripID: id.String(), Latitude: 59.93, Longitude: 30.33, RecordedAt: time.Now()}, nil)
	waiting.EXPECT().Forget(id.String())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/positions", strings.NewReader(
		`{"coordinates":{"latitude":59.93,"longitude":30.33},"recorded_at":"`+recorded+`"}`,
	))
	rec := httptest.NewRecorder()
	h.CreateTripPosition(rec, req, id)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateTripPositionMissingFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := save.NewHandler(NewMockPointSaver(ctrl), NewMockWaitingTrips(ctrl), slog.New(slog.DiscardHandler))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"recorded_at":"2026-11-01T10:00:00Z"}`))
	rec := httptest.NewRecorder()
	h.CreateTripPosition(rec, req, uuid.New())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateTripPositionBadBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := save.NewHandler(NewMockPointSaver(ctrl), NewMockWaitingTrips(ctrl), slog.New(slog.DiscardHandler))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"extra":true}`))
	rec := httptest.NewRecorder()
	h.CreateTripPosition(rec, req, uuid.New())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestCreateTripPositionCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	points := NewMockPointSaver(ctrl)
	h := save.NewHandler(points, NewMockWaitingTrips(ctrl), slog.New(slog.DiscardHandler))
	id := uuid.New()
	points.EXPECT().Save(gomock.Any(), id.String(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(trip.Point{}, trip.ErrTripCompleted)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		`{"coordinates":{"latitude":1,"longitude":2},"recorded_at":"2026-11-01T10:00:00Z"}`,
	))
	rec := httptest.NewRecorder()
	h.CreateTripPosition(rec, req, id)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "trip_completed") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

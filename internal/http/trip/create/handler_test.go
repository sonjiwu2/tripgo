package create_test

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/api"
	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/http/trip/create"
)

func TestCreateTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	trips := NewMockTripCreator(ctrl)
	handler := create.NewHandler(trips, slog.New(slog.DiscardHandler))
	item := mustTrip(t)
	trips.EXPECT().Create(gomock.Any(), gomock.Any()).Return(item, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body()))
	rec := httptest.NewRecorder()
	handler.CreateTrip(rec, req, api.CreateTripParams{})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/api/v1/trips/"+item.ID {
		t.Fatalf("location %s", rec.Header().Get("Location"))
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type %s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"status":"active"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestCreateTripDriverBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	trips := NewMockTripCreator(ctrl)
	handler := create.NewHandler(trips, slog.New(slog.DiscardHandler))
	trips.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domaintrip.Trip{}, domaintrip.ErrDriverBusy)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body()))
	rec := httptest.NewRecorder()
	handler.CreateTrip(rec, req, api.CreateTripParams{})

	assertProblem(t, rec, http.StatusConflict, "driver_busy")
}

func TestCreateTripBadBody(t *testing.T) {
	handler := create.NewHandler(NewMockTripCreator(gomock.NewController(t)), slog.New(slog.DiscardHandler))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(`{"price":`))
	rec := httptest.NewRecorder()
	handler.CreateTrip(rec, req, api.CreateTripParams{})
	assertProblem(t, rec, http.StatusBadRequest, "invalid_request")
}

func TestCreateTripRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	trips := NewMockTripCreator(ctrl)
	handler := create.NewHandler(trips, slog.New(slog.DiscardHandler))
	trips.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domaintrip.Trip{}, errors.New("postgres: password=secret"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body()))
	rec := httptest.NewRecorder()
	handler.CreateTrip(rec, req, api.CreateTripParams{})

	assertProblem(t, rec, http.StatusInternalServerError, "internal_error")
	if strings.Contains(rec.Body.String(), "password=secret") {
		t.Fatalf("leaked: %s", rec.Body.String())
	}
}

func assertProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Fatalf("content-type %s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
	if code == "invalid_request" && !strings.Contains(rec.Body.String(), "https://tripgo.example/problems/invalid-request") {
		t.Fatalf("type %s", rec.Body.String())
	}
}

func body() string {
	return `{
		"user_id":"` + uuid.NewString() + `",
		"driver_id":"` + uuid.NewString() + `",
		"start_point":{"latitude":59.9,"longitude":30.3},
		"end_point":{"latitude":59.8,"longitude":30.2},
		"price":100
	}`
}

func mustTrip(t *testing.T) domaintrip.Trip {
	t.Helper()
	item, err := domaintrip.New(uuid.NewString(), uuid.NewString(), uuid.NewString(), 59.9, 30.3, 59.8, 30.2, 100, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return item
}

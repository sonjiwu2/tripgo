package get_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonjiwu2/tripgo/api"
	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/http/trip/get"
)

func TestGetTripMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	trips := NewMockTripReader(ctrl)
	handler := get.NewHandler(trips, slog.New(slog.DiscardHandler))
	id := uuid.New()
	trips.EXPECT().Get(gomock.Any(), id.String()).Return(domaintrip.Trip{}, domaintrip.ErrTripNotFound)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)
	rec := httptest.NewRecorder()
	handler.GetTrip(rec, req, api.TripId(id))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"trip_not_found"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

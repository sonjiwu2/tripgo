package finish_test

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
	"github.com/sonjiwu2/tripgo/internal/http/trip/finish"
)

func TestFinishTripCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	trips := NewMockTripFinisher(ctrl)
	handler := finish.NewHandler(trips, slog.New(slog.DiscardHandler))
	id := uuid.New()
	trips.EXPECT().Finish(gomock.Any(), id.String()).Return(domaintrip.Trip{}, domaintrip.ErrTripCompleted)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
	rec := httptest.NewRecorder()
	handler.FinishTrip(rec, req, api.TripId(id))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"trip_completed"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

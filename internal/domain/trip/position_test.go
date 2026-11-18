package trip

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewPoint(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	id := uuid.NewString()

	tests := []struct {
		name     string
		id       string
		lat      float64
		lon      float64
		recorded time.Time
		field    string
	}{
		{name: "ok", id: id, lat: 59.93, lon: 30.33, recorded: now},
		{name: "edges", id: id, lat: 90, lon: -180, recorded: now.Add(time.Minute)},
		{name: "bad id", id: "поездка", lat: 1, lon: 1, recorded: now, field: "trip_id"},
		{name: "bad lat", id: id, lat: 90.1, lon: 0, recorded: now, field: "coordinates"},
		{name: "bad lon", id: id, lat: 0, lon: 180.1, recorded: now, field: "coordinates"},
		{name: "empty time", id: id, lat: 1, lon: 1, field: "recorded_at"},
		{name: "future", id: id, lat: 1, lon: 1, recorded: now.Add(time.Minute + time.Second), field: "recorded_at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPoint(tt.id, tt.lat, tt.lon, tt.recorded, now)
			if tt.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				if got.TripID != tt.id || got.Latitude != tt.lat || got.Longitude != tt.lon {
					t.Fatalf("point %+v", got)
				}
				if !got.RecordedAt.Equal(tt.recorded) {
					t.Fatalf("recorded %s", got.RecordedAt)
				}
				return
			}
			var field *FieldError
			if !errors.As(err, &field) {
				t.Fatalf("err %v", err)
			}
			if field.Field != tt.field {
				t.Fatalf("field %s", field.Field)
			}
		})
	}
}

package trip

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	id := uuid.NewString()
	user := uuid.NewString()
	driver := uuid.NewString()

	tests := []struct {
		name   string
		id     string
		user   string
		driver string
		lat    float64
		lon    float64
		elat   float64
		elon   float64
		price  int64
		field  string
	}{
		{name: "ok", id: id, user: user, driver: driver, lat: 59.9, lon: 30.3, elat: -33.8, elon: 151.2, price: 0},
		{name: "edges", id: id, user: user, driver: driver, lat: 90, lon: -180, elat: -90, elon: 180, price: 10},
		{name: "empty id", user: user, driver: driver, lat: 1, lon: 1, elat: 2, elon: 2, field: "id"},
		{name: "nil user", id: id, user: uuid.Nil.String(), driver: driver, lat: 1, lon: 1, elat: 2, elon: 2, field: "user_id"},
		{name: "bad driver", id: id, user: user, driver: "водитель", lat: 1, lon: 1, elat: 2, elon: 2, field: "driver_id"},
		{name: "negative price", id: id, user: user, driver: driver, lat: 1, lon: 1, elat: 2, elon: 2, price: -1, field: "price"},
		{name: "bad start", id: id, user: user, driver: driver, lat: 90.1, lon: 0, elat: 2, elon: 2, field: "start_point"},
		{name: "bad end", id: id, user: user, driver: driver, lat: 1, lon: 1, elat: 0, elon: 180.1, field: "end_point"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.id, tt.user, tt.driver, tt.lat, tt.lon, tt.elat, tt.elon, tt.price, at)
			if tt.field == "" {
				if err != nil {
					t.Fatalf("New: %v", err)
				}
				if got.Status != StatusActive {
					t.Fatalf("status %s", got.Status)
				}
				if !got.StartedAt.Equal(at) || !got.CreatedAt.Equal(at) {
					t.Fatalf("timestamps %+v", got)
				}
				if !got.UpdatedAt.Equal(at) {
					t.Fatalf("updated_at %s", got.UpdatedAt)
				}
				if got.StartLatitude != tt.lat || got.StartLongitude != tt.lon {
					t.Fatalf("start %+v", got)
				}
				if got.EndLatitude != tt.elat || got.EndLongitude != tt.elon {
					t.Fatalf("end %+v", got)
				}
				if got.FinishedAt != nil {
					t.Fatal("finished_at is set")
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

func TestFieldErrorText(t *testing.T) {
	err := &FieldError{Field: "price", Reason: "цена не может быть отрицательной"}
	if err.Error() != "price: цена не может быть отрицательной" {
		t.Fatalf("text %s", err.Error())
	}
}

func TestFinish(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	item, err := New(uuid.NewString(), uuid.NewString(), uuid.NewString(), 1, 2, 3, 4, 100, at)
	if err != nil {
		t.Fatal(err)
	}

	doneAt := at.Add(time.Hour)
	if err = item.Finish(doneAt); err != nil {
		t.Fatal(err)
	}
	if item.Status != StatusCompleted {
		t.Fatalf("status %s", item.Status)
	}
	if item.FinishedAt == nil || !item.FinishedAt.Equal(doneAt) {
		t.Fatalf("finished_at %+v", item.FinishedAt)
	}
	if !item.UpdatedAt.Equal(doneAt) {
		t.Fatalf("updated_at %s", item.UpdatedAt)
	}

	if err = item.Finish(doneAt.Add(time.Minute)); !errors.Is(err, ErrTripCompleted) {
		t.Fatalf("second finish: %v", err)
	}
	if item.FinishedAt == nil || !item.FinishedAt.Equal(doneAt) {
		t.Fatal("finish time changed")
	}
}

package trip

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Trip struct {
	ID             string
	UserID         string
	DriverID       string
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         Status
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func New(id, userID, driverID string, startLat, startLon, endLat, endLon float64, price int64, at time.Time) (Trip, error) {
	if err := tripDataOK(id, userID, driverID, price, startLat, startLon, endLat, endLon); err != nil {
		return Trip{}, err
	}
	return Trip{
		ID:             id,
		UserID:         userID,
		DriverID:       driverID,
		StartLatitude:  startLat,
		StartLongitude: startLon,
		EndLatitude:    endLat,
		EndLongitude:   endLon,
		Price:          price,
		Status:         StatusActive,
		StartedAt:      at,
		CreatedAt:      at,
		UpdatedAt:      at,
	}, nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}
	t.Status = StatusCompleted
	t.FinishedAt = &at
	t.UpdatedAt = at
	return nil
}

func tripDataOK(id, userID, driverID string, price int64, startLat, startLon, endLat, endLon float64) error {
	if !idOK(id) {
		return &FieldError{Field: "id", Reason: "некорректный id поездки"}
	}
	if !idOK(userID) {
		return &FieldError{Field: "user_id", Reason: "некорректный id пользователя"}
	}
	if !idOK(driverID) {
		return &FieldError{Field: "driver_id", Reason: "некорректный id водителя"}
	}
	if price < 0 {
		return &FieldError{Field: "price", Reason: "цена не может быть отрицательной"}
	}
	if !coordsOK(startLat, startLon) {
		return &FieldError{Field: "start_point", Reason: "координаты старта вне диапазона"}
	}
	if !coordsOK(endLat, endLon) {
		return &FieldError{Field: "end_point", Reason: "координаты финиша вне диапазона"}
	}
	return nil
}

func idOK(raw string) bool {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return false
	}
	return parsed != uuid.Nil
}

func coordsOK(lat, lon float64) bool {
	if lat < -90 || lat > 90 {
		return false
	}
	return lon >= -180 && lon <= 180
}

package trip

import "time"

type Point struct {
	ID         int64
	TripID     string
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

type Quiet struct {
	ID       string
	DriverID string
	Since    time.Time
}

func NewPoint(tripID string, lat, lon float64, recorded, now time.Time) (Point, error) {
	if !idOK(tripID) {
		return Point{}, &FieldError{Field: "trip_id", Reason: "некорректный id поездки"}
	}
	if !coordsOK(lat, lon) {
		return Point{}, &FieldError{Field: "coordinates", Reason: "координаты вне диапазона"}
	}
	if recorded.IsZero() {
		return Point{}, &FieldError{Field: "recorded_at", Reason: "время координаты не задано"}
	}
	if recorded.After(now.Add(time.Minute)) {
		return Point{}, &FieldError{Field: "recorded_at", Reason: "время координаты слишком далеко в будущем"}
	}
	return Point{
		TripID:     tripID,
		Latitude:   lat,
		Longitude:  lon,
		RecordedAt: recorded,
	}, nil
}

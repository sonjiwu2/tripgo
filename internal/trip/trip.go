package trip

import (
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("такой поездки нет")
	ErrCompleted  = errors.New("эта поездка уже завершена")
	ErrDriverBusy = errors.New("у водителя уже есть активная поездка")
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
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

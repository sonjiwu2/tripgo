package model

type Draft struct {
	UserID         string
	DriverID       string
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
}

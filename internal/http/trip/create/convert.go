package create

import (
	"github.com/google/uuid"

	"github.com/sonjiwu2/tripgo/internal/model"
)

type tripRequest struct {
	UserID     *uuid.UUID    `json:"user_id"`
	DriverID   *uuid.UUID    `json:"driver_id"`
	StartPoint *pointRequest `json:"start_point"`
	EndPoint   *pointRequest `json:"end_point"`
	Price      *int64        `json:"price"`
}

type pointRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

func (r tripRequest) complete() bool {
	return r.UserID != nil && r.DriverID != nil && r.Price != nil &&
		r.StartPoint.complete() && r.EndPoint.complete()
}

func (p *pointRequest) complete() bool {
	return p != nil && p.Latitude != nil && p.Longitude != nil
}

func draftFrom(body tripRequest) model.Draft {
	return model.Draft{
		UserID:         body.UserID.String(),
		DriverID:       body.DriverID.String(),
		StartLatitude:  *body.StartPoint.Latitude,
		StartLongitude: *body.StartPoint.Longitude,
		EndLatitude:    *body.EndPoint.Latitude,
		EndLongitude:   *body.EndPoint.Longitude,
		Price:          *body.Price,
	}
}

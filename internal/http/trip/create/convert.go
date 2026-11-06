package create

import (
	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/model"
)

func draftFrom(body api.TripData) model.Draft {
	return model.Draft{
		UserID:         body.UserId.String(),
		DriverID:       body.DriverId.String(),
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
	}
}

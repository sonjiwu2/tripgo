package present

import (
	"github.com/google/uuid"

	"github.com/sonjiwu2/tripgo/api"
	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

func Trip(item domaintrip.Trip) api.Trip {
	id, _ := uuid.Parse(item.ID)
	user, _ := uuid.Parse(item.UserID)
	driver, _ := uuid.Parse(item.DriverID)
	return api.Trip{
		Id:         id,
		UserId:     user,
		DriverId:   driver,
		StartPoint: api.Coordinates{Latitude: item.StartLatitude, Longitude: item.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: item.EndLatitude, Longitude: item.EndLongitude},
		Price:      item.Price,
		Status:     api.TripStatus(item.Status),
		StartedAt:  item.StartedAt,
		FinishedAt: item.FinishedAt,
	}
}

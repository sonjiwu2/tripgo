package domainerror

import (
	"errors"
	"net/http"
	"strings"

	"github.com/sonjiwu2/tripgo/api"
	"github.com/sonjiwu2/tripgo/internal/domain/trip"
)

func Invalid(detail string) (int, api.Problem) {
	return problem(http.StatusBadRequest, "invalid_request", "Invalid request", detail)
}

func Problem(err error) (int, api.Problem) {
	var field *trip.FieldError
	switch {
	case errors.As(err, &field):
		return problem(http.StatusBadRequest, "invalid_request", "Invalid request", field.Error())
	case errors.Is(err, trip.ErrTripNotFound):
		return problem(http.StatusNotFound, "trip_not_found", "Trip not found", "такой поездки нет")
	case errors.Is(err, trip.ErrTripCompleted):
		return problem(http.StatusConflict, "trip_completed", "Trip completed", "эта поездка уже завершена")
	case errors.Is(err, trip.ErrDriverBusy):
		return problem(http.StatusConflict, "driver_busy", "Driver busy", "у водителя уже есть активная поездка")
	default:
		return problem(http.StatusInternalServerError, "internal_error", "Internal error", "не получилось обработать запрос")
	}
}

func problem(status int32, code, title, detail string) (int, api.Problem) {
	return int(status), api.Problem{
		Type:   "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"),
		Title:  title,
		Status: status,
		Code:   code,
		Detail: &detail,
	}
}

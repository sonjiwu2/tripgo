package trip

import "errors"

var (
	ErrTripNotFound  = errors.New("такой поездки нет")
	ErrTripCompleted = errors.New("эта поездка уже завершена")
	ErrDriverBusy    = errors.New("у водителя уже есть активная поездка")
)

type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string {
	return e.Field + ": " + e.Reason
}

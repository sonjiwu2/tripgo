package domainerror

import (
	"errors"
	"net/http"
	"testing"

	"github.com/sonjiwu2/tripgo/internal/domain/trip"
)

func TestProblem(t *testing.T) {
	secret := errors.New("postgres: password=secret")
	tests := []struct {
		name   string
		err    error
		status int
		code   string
		type_  string
	}{
		{
			name:   "field",
			err:    &trip.FieldError{Field: "price", Reason: "цена не может быть отрицательной"},
			status: http.StatusBadRequest,
			code:   "invalid_request",
			type_:  "https://tripgo.example/problems/invalid-request",
		},
		{
			name:   "missing",
			err:    trip.ErrTripNotFound,
			status: http.StatusNotFound,
			code:   "trip_not_found",
			type_:  "https://tripgo.example/problems/trip-not-found",
		},
		{
			name:   "completed",
			err:    trip.ErrTripCompleted,
			status: http.StatusConflict,
			code:   "trip_completed",
			type_:  "https://tripgo.example/problems/trip-completed",
		},
		{
			name:   "busy",
			err:    trip.ErrDriverBusy,
			status: http.StatusConflict,
			code:   "driver_busy",
			type_:  "https://tripgo.example/problems/driver-busy",
		},
		{
			name:   "internal",
			err:    secret,
			status: http.StatusInternalServerError,
			code:   "internal_error",
			type_:  "https://tripgo.example/problems/internal-error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, problem := Problem(tt.err)
			if status != tt.status {
				t.Fatalf("status %d", status)
			}
			if problem.Code != tt.code {
				t.Fatalf("code %s", problem.Code)
			}
			if problem.Type != tt.type_ {
				t.Fatalf("type %s", problem.Type)
			}
			if problem.Detail == nil {
				t.Fatal("empty detail")
			}
			if tt.name == "internal" && *problem.Detail == secret.Error() {
				t.Fatalf("detail leaked: %s", *problem.Detail)
			}
			if tt.name == "field" && *problem.Detail != "price: цена не может быть отрицательной" {
				t.Fatalf("detail %s", *problem.Detail)
			}
		})
	}
}

func TestInvalid(t *testing.T) {
	status, problem := Invalid("тело запроса не разобрать")
	if status != http.StatusBadRequest {
		t.Fatalf("status %d", status)
	}
	if problem.Code != "invalid_request" {
		t.Fatalf("code %s", problem.Code)
	}
	if problem.Type != "https://tripgo.example/problems/invalid-request" {
		t.Fatalf("type %s", problem.Type)
	}
}

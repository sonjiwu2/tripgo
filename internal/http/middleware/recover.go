package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/sonjiwu2/tripgo/internal/http/domainerror"
	"github.com/sonjiwu2/tripgo/internal/http/respond"
)

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				got := recover()
				if got == nil {
					return
				}
				log.ErrorContext(r.Context(), "паника в обработчике", "panic", got, "stack", string(debug.Stack()))
				status, problem := domainerror.Problem(errPanic{})
				path := r.URL.Path
				problem.Instance = &path
				respond.Problem(w, status, problem)
				logHandled(log, r, http.StatusInternalServerError, 0)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type errPanic struct{}

func (errPanic) Error() string {
	return "паника"
}

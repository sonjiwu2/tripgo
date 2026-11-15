package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := newCapture(w)
			next.ServeHTTP(rec, r)
			logHandled(log, r, rec.status, time.Since(start).Milliseconds())
		})
	}
}

func logHandled(log *slog.Logger, r *http.Request, status int, durationMS int64) {
	level := slog.LevelInfo
	if status >= http.StatusInternalServerError {
		level = slog.LevelError
	}
	log.Log(r.Context(), level, "request handled",
		"method", r.Method,
		"route", Pattern(r),
		"status", status,
		"duration_ms", durationMS,
	)
}

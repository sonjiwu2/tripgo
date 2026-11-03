package telemetry

import (
	"log/slog"
	"os"
)

func NewLogger(level slog.Level, service string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With("service", service)
}

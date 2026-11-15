package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sonjiwu2/tripgo/internal/config"
	"github.com/sonjiwu2/tripgo/internal/container"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("конфиг не прочитан", "err", err)
		os.Exit(1)
	}

	os.Exit(run(cfg))
}

func run(cfg config.Config) int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := container.New(ctx, cfg)
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("база данных недоступна", "err", err)
		return 1
	}

	listenErr := make(chan error, 1)
	go func() {
		app.Log.Info("сервер запущен", "addr", cfg.HTTP.Addr)
		listenErr <- app.Server.ListenAndServe()
	}()

	var serverErr error
	select {
	case <-ctx.Done():
	case serverErr = <-listenErr:
	}

	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	defer func() {
		if err := app.Close(shutdownCtx); err != nil {
			app.Log.Error("не получилось закрыть сервис", "err", err)
		}
	}()

	if serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
		app.Log.Error("сервер не запустился", "err", serverErr)
		return 1
	}

	app.Log.Info("сервер останавливается")
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		app.Log.Error("не уложились в SHUTDOWN_TIMEOUT, обрываю соединения", "err", err)
		_ = app.Server.Close()
		return 1
	}
	return 0
}

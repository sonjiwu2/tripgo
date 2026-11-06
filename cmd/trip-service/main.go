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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := container.New(ctx, cfg)
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("база данных недоступна", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := app.Close(closeCtx); err != nil {
			app.Log.Error("не получилось закрыть сервис", "err", err)
		}
	}()

	listenErr := make(chan error, 1)
	go func() {
		app.Log.Info("сервер запущен", "addr", cfg.HTTP.Addr)
		listenErr <- app.Server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-listenErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Log.Error("сервер не запустился", "err", err)
			_ = app.Close(context.Background())
			os.Exit(1)
		}
		return
	}

	stop()
	app.Log.Info("сервер останавливается")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		app.Log.Error("не уложились в SHUTDOWN_TIMEOUT, обрываю соединения", "err", err)
		_ = app.Server.Close()
		_ = app.Close(context.Background())
		os.Exit(1)
	}
}

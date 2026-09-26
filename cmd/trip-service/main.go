package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sonjiwu2/tripgo/internal/config"
	"github.com/sonjiwu2/tripgo/internal/httpapi"
	"github.com/sonjiwu2/tripgo/internal/postgres"
	"github.com/sonjiwu2/tripgo/internal/trip"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("конфиг не прочитан", "err", err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.Log.Level,
	})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		slog.Error("база данных недоступна", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := trip.NewRepo(pool)
	handler := httpapi.NewHandler(repo, postgres.NewTxManager(pool), pool)
	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           httpapi.Routes(handler),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Info("сервер запущен", "addr", cfg.HTTP.Addr)
		listenErr <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-listenErr:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("сервер не запустился", "err", err)
		}
		return
	}

	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("не дождался завершения запросов", "err", err)
	}
}

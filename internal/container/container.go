package container

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/sonjiwu2/tripgo/internal/config"
	"github.com/sonjiwu2/tripgo/internal/http/probe"
	"github.com/sonjiwu2/tripgo/internal/http/router"
	triphttp "github.com/sonjiwu2/tripgo/internal/http/trip/create"
	finishhttp "github.com/sonjiwu2/tripgo/internal/http/trip/finish"
	gethttp "github.com/sonjiwu2/tripgo/internal/http/trip/get"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres/transaction"
	"github.com/sonjiwu2/tripgo/internal/infra/telemetry"
	triprepo "github.com/sonjiwu2/tripgo/internal/repository/trip"
	tripcreate "github.com/sonjiwu2/tripgo/internal/usecase/trip/create"
	tripfinish "github.com/sonjiwu2/tripgo/internal/usecase/trip/finish"
	tripget "github.com/sonjiwu2/tripgo/internal/usecase/trip/get"
)

type App struct {
	Server *http.Server
	Log    *slog.Logger

	once  sync.Once
	err   error
	close func(context.Context) error
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	log := telemetry.NewLogger(cfg.Log.Level, cfg.OTEL.ServiceName)
	shutdown, err := telemetry.Setup(ctx, cfg.OTEL)
	if err != nil {
		log.Error("телеметрия не поднялась, продолжаю без неё", "err", err)
		shutdown = func(context.Context) error { return nil }
	}

	meters, err := telemetry.NewMeters()
	if err != nil {
		_ = shutdown(context.Background())
		return nil, err
	}

	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		_ = shutdown(context.Background())
		return nil, err
	}

	repo := triprepo.New(pool, cfg.DB.QueryTimeout)
	if err = telemetry.WatchActive(log, repo); err != nil {
		log.Error("счётчик активных поездок не включился", "err", err)
	}

	tx := transaction.NewTxManager(pool)
	created := tripcreate.New(repo, tx, log, meters.Created)
	finished := tripfinish.New(repo, tx, log, meters.Completed)
	loaded := tripget.New(repo)
	handler := router.New(
		triphttp.NewHandler(created, log),
		gethttp.NewHandler(loaded, log),
		finishhttp.NewHandler(finished, log),
		probe.New(pool, cfg.DB.QueryTimeout),
	)

	return &App{
		Log: log,
		Server: &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           router.Routes(handler, log, meters.HTTPDuration, meters.HTTPActive),
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
		},
		close: func(ctx context.Context) error {
			pool.Close()
			return shutdown(ctx)
		},
	}, nil
}

func (a *App) Close(ctx context.Context) error {
	a.once.Do(func() {
		if a.close != nil {
			a.err = a.close(ctx)
		}
	})
	return a.err
}

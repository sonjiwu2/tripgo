package container

import (
	"context"
	"log/slog"
	"net/http"

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
	close  func()
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	log := telemetry.NewLogger(cfg.Log.Level, "trip-service")

	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	repo := triprepo.New(pool, cfg.DB.QueryTimeout)
	tx := transaction.NewTxManager(pool)
	created := tripcreate.New(repo, tx, log, quietCounter{})
	finished := tripfinish.New(repo, tx, log, quietCounter{})
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
			Handler:           router.Routes(handler, log),
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
		},
		close: pool.Close,
	}, nil
}

func (a *App) Close() {
	if a.close != nil {
		a.close()
	}
}

type quietCounter struct{}

func (quietCounter) Add(context.Context, int64) {}

package container

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/sonjiwu2/tripgo/internal/config"
	"github.com/sonjiwu2/tripgo/internal/gateway/push"
	poslist "github.com/sonjiwu2/tripgo/internal/http/position/list"
	possave "github.com/sonjiwu2/tripgo/internal/http/position/save"
	"github.com/sonjiwu2/tripgo/internal/http/probe"
	"github.com/sonjiwu2/tripgo/internal/http/router"
	triphttp "github.com/sonjiwu2/tripgo/internal/http/trip/create"
	finishhttp "github.com/sonjiwu2/tripgo/internal/http/trip/finish"
	gethttp "github.com/sonjiwu2/tripgo/internal/http/trip/get"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres/transaction"
	"github.com/sonjiwu2/tripgo/internal/infra/telemetry"
	positionrepo "github.com/sonjiwu2/tripgo/internal/repository/position"
	triprepo "github.com/sonjiwu2/tripgo/internal/repository/trip"
	poslistuc "github.com/sonjiwu2/tripgo/internal/usecase/position/list"
	possaveuc "github.com/sonjiwu2/tripgo/internal/usecase/position/save"
	tripcreate "github.com/sonjiwu2/tripgo/internal/usecase/trip/create"
	tripfinish "github.com/sonjiwu2/tripgo/internal/usecase/trip/finish"
	tripget "github.com/sonjiwu2/tripgo/internal/usecase/trip/get"
	"github.com/sonjiwu2/tripgo/internal/worker/positions"
)

type App struct {
	Server    *http.Server
	Log       *slog.Logger
	positions *positions.Pool

	once  sync.Once
	err   error
	close func(context.Context) error
}

func New(ctx, work context.Context, cfg config.Config) (*App, error) {
	log := telemetry.NewLogger(cfg.Log.Level, cfg.OTEL.ServiceName)
	shutdown, err := telemetry.Setup(ctx, cfg.OTEL)
	if err != nil {
		log.Error("телеметрия не поднялась, продолжаю без неё", "err", err)
		shutdown = func(context.Context) error { return nil }
	}

	meters, err := telemetry.NewMeters()
	if err != nil {
		_ = shutdown(ctx)
		return nil, err
	}

	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		_ = shutdown(ctx)
		return nil, err
	}

	repo := triprepo.New(pool, cfg.DB.QueryTimeout)
	if err = telemetry.WatchActive(log, repo); err != nil {
		log.Error("счётчик активных поездок не включился", "err", err)
	}
	points := positionrepo.New(pool, cfg.DB.QueryTimeout)

	tx := transaction.NewTxManager(pool, cfg.DB.QueryTimeout)
	created := tripcreate.New(repo, tx, log, meters.Created)
	finished := tripfinish.New(repo, tx, log, meters.Completed)
	loaded := tripget.New(repo)
	saved := possaveuc.New(points, tx, meters.PositionsSaved)
	listed := poslistuc.New(points, repo)
	gateway := push.New(cfg.Push.URL, cfg.Push.Timeout)
	poller := positions.New(log, points, gateway, positions.Config{
		Interval:    cfg.Position.CheckInterval,
		Stale:       cfg.Position.StaleAfter,
		Workers:     cfg.Position.Workers,
		Queue:       cfg.Position.QueueSize,
		BatchSize:   cfg.Position.ScanBatchSize,
		PushTimeout: cfg.Push.Timeout,
	}, meters.StaleChecks, meters.PositionRequests, meters.PositionDuration, meters.RowsRead)
	if err = telemetry.WatchPositions(poller); err != nil {
		log.Error("метрики опроса координат не включились", "err", err)
	}
	poller.Start(work)
	handler := router.New(
		triphttp.NewHandler(created, log),
		gethttp.NewHandler(loaded, log),
		finishhttp.NewHandler(finished, log),
		possave.NewHandler(saved, poller, log),
		poslist.NewHandler(listed, log),
		probe.New(pool, cfg.DB.QueryTimeout),
	)

	return &App{
		Log:       log,
		positions: poller,
		Server: &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           router.Routes(handler, log, meters.HTTPDuration, meters.HTTPActive),
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
		},
		close: func(ctx context.Context) error {
			err := shutdown(ctx)
			gateway.Close()
			pool.Close()
			return err
		},
	}, nil
}

func (a *App) Stop(ctx context.Context) error {
	if a.positions == nil {
		return nil
	}
	return a.positions.Stop(ctx)
}

func (a *App) Close(ctx context.Context) error {
	a.once.Do(func() {
		if a.close != nil {
			a.err = a.close(ctx)
		}
	})
	return a.err
}

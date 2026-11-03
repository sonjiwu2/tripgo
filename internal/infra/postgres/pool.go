package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonjiwu2/tripgo/internal/config"
)

func NewPool(ctx context.Context, db config.DB) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(db.URL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL указан неверно: %w", err)
	}
	cfg.MaxConns = db.MaxConns
	cfg.MinConns = db.MinConns
	cfg.MaxConnLifetime = db.MaxConnLifetime
	cfg.ConnConfig.ConnectTimeout = db.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("не удалось запустить пул соединений: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, db.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("нет подключения к базе: %w", err)
	}

	return pool, nil
}

package position

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonjiwu2/tripgo/internal/infra/postgres/transaction"
)

type Repo struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func New(pool *pgxpool.Pool, queryTimeout time.Duration) *Repo {
	return &Repo{pool: pool, queryTimeout: queryTimeout}
}

func (r *Repo) save(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	if tx, ok := transaction.Current(ctx); ok {
		return tx.Exec(ctx, query, args...)
	}
	return r.pool.Exec(ctx, query, args...)
}

type scanned struct {
	pgx.Row
	stop context.CancelFunc
}

func (s scanned) Scan(dest ...any) error {
	defer s.stop()
	return s.Row.Scan(dest...)
}

func (r *Repo) load(ctx context.Context, query string, args ...any) pgx.Row {
	ctx, stop := context.WithTimeout(ctx, r.queryTimeout)
	if tx, ok := transaction.Current(ctx); ok {
		return scanned{Row: tx.QueryRow(ctx, query, args...), stop: stop}
	}
	return scanned{Row: r.pool.QueryRow(ctx, query, args...), stop: stop}
}

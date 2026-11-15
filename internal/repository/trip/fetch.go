package trip

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres/transaction"
)

func (r *Repo) Find(ctx context.Context, id string) (domaintrip.Trip, error) {
	query, args, err := sqlb.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at", "created_at", "updated_at",
	).From("trips").Where("id = ?", id).ToSql()
	if err != nil {
		return domaintrip.Trip{}, err
	}

	var found domaintrip.Trip
	err = r.load(ctx, query, args...).Scan(
		&found.ID, &found.UserID, &found.DriverID,
		&found.StartLatitude, &found.StartLongitude, &found.EndLatitude, &found.EndLongitude,
		&found.Price, &found.Status, &found.StartedAt, &found.FinishedAt, &found.CreatedAt, &found.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Trip{}, domaintrip.ErrTripNotFound
	}
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("не получилось прочитать поездку из базы: %w", err)
	}
	return found, nil
}

func (r *Repo) CountActive(ctx context.Context) (int64, error) {
	query, args, err := sqlb.Select("count(*)").From("trips").Where("status = ?", domaintrip.StatusActive).ToSql()
	if err != nil {
		return 0, err
	}
	var n int64
	if err = r.load(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("не получилось посчитать активные поездки: %w", err)
	}
	return n, nil
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

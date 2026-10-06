package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonjiwu2/tripgo/internal/trip"
)

var sqlb = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type Repo struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepo(pool *pgxpool.Pool, queryTimeout time.Duration) *Repo {
	return &Repo{pool: pool, queryTimeout: queryTimeout}
}

func (r *Repo) save(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	if tx, ok := getTransaction(ctx); ok {
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
	if tx, ok := getTransaction(ctx); ok {
		return scanned{Row: tx.QueryRow(ctx, query, args...), stop: stop}
	}
	return scanned{Row: r.pool.QueryRow(ctx, query, args...), stop: stop}
}

func (r *Repo) Create(ctx context.Context, item trip.Trip) (trip.Trip, error) {
	moment := time.Now()
	cols := []string{
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status", "started_at",
	}
	vals := []any{
		item.ID, item.UserID, item.DriverID,
		item.StartLatitude, item.StartLongitude, item.EndLatitude, item.EndLongitude,
		item.Price, "active", moment,
	}

	query, args, err := sqlb.Insert("trips").Columns(cols...).Values(vals...).ToSql()
	if err != nil {
		return trip.Trip{}, err
	}
	_, err = r.save(ctx, query, args...)
	if err != nil {
		if driverIsBusy(err) {
			return trip.Trip{}, trip.ErrDriverBusy
		}
		return trip.Trip{}, fmt.Errorf("не получилось добавить поездку в базу: %w", err)
	}

	query, args, err = sqlb.Insert("trip_status_history").
		Columns("trip_id", "to_status").
		Values(item.ID, "active").
		ToSql()
	if err != nil {
		return trip.Trip{}, err
	}
	_, err = r.save(ctx, query, args...)
	if err != nil {
		return trip.Trip{}, fmt.Errorf("поездка создана, но статус в журнал не записался: %w", err)
	}

	return r.Get(ctx, item.ID)
}

func (r *Repo) Get(ctx context.Context, id string) (trip.Trip, error) {
	query, args, err := sqlb.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at", "created_at", "updated_at",
	).From("trips").Where("id = ?", id).ToSql()
	if err != nil {
		return trip.Trip{}, err
	}

	var found trip.Trip
	err = r.load(ctx, query, args...).Scan(
		&found.ID, &found.UserID, &found.DriverID,
		&found.StartLatitude, &found.StartLongitude, &found.EndLatitude, &found.EndLongitude,
		&found.Price, &found.Status, &found.StartedAt, &found.FinishedAt, &found.CreatedAt, &found.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return trip.Trip{}, trip.ErrNotFound
	}
	if err != nil {
		return trip.Trip{}, fmt.Errorf("не получилось прочитать поездку из базы: %w", err)
	}
	return found, nil
}

func (r *Repo) Finish(ctx context.Context, id string) (trip.Trip, error) {
	moment := time.Now()
	query, args, err := sqlb.Update("trips").
		Set("status", "completed").
		Set("finished_at", moment).
		Set("updated_at", moment).
		Where("id = ? AND status = ?", id, "active").
		ToSql()
	if err != nil {
		return trip.Trip{}, err
	}

	tag, err := r.save(ctx, query, args...)
	if err != nil {
		return trip.Trip{}, fmt.Errorf("не получилось закрыть поездку: %w", err)
	}
	if tag.RowsAffected() == 0 {
		found, readErr := r.Get(ctx, id)
		if readErr != nil {
			return trip.Trip{}, readErr
		}
		if found.Status == "completed" {
			return trip.Trip{}, trip.ErrCompleted
		}
		return trip.Trip{}, fmt.Errorf("поездка нашлась, но закрыть её не вышло")
	}

	query, args, err = sqlb.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status").
		Values(id, "active", "completed").
		ToSql()
	if err != nil {
		return trip.Trip{}, err
	}
	_, err = r.save(ctx, query, args...)
	if err != nil {
		return trip.Trip{}, fmt.Errorf("поездка закрыта, но запись в журнал не попала: %w", err)
	}

	return r.Get(ctx, id)
}

func driverIsBusy(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_driver_idx"
}

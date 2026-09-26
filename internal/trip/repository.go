package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonjiwu2/tripgo/internal/postgres"
)

var (
	ErrNotFound   = errors.New("такой поездки нет")
	ErrCompleted  = errors.New("эта поездка уже завершена")
	ErrDriverBusy = errors.New("у водителя уже есть активная поездка")

	sqlb = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type Trip struct {
	ID             string
	UserID         string
	DriverID       string
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) save(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := postgres.GetTransaction(ctx); ok {
		return tx.Exec(ctx, query, args...)
	}
	return r.pool.Exec(ctx, query, args...)
}

func (r *Repo) load(ctx context.Context, query string, args ...any) pgx.Row {
	if tx, ok := postgres.GetTransaction(ctx); ok {
		return tx.QueryRow(ctx, query, args...)
	}
	return r.pool.QueryRow(ctx, query, args...)
}

func (r *Repo) Create(ctx context.Context, trip Trip) (Trip, error) {
	moment := time.Now()
	cols := []string{
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status", "started_at",
	}
	vals := []any{
		trip.ID, trip.UserID, trip.DriverID,
		trip.StartLatitude, trip.StartLongitude, trip.EndLatitude, trip.EndLongitude,
		trip.Price, "active", moment,
	}

	query, args, err := sqlb.Insert("trips").Columns(cols...).Values(vals...).ToSql()
	if err != nil {
		return Trip{}, err
	}
	_, err = r.save(ctx, query, args...)
	if err != nil {
		if driverIsBusy(err) {
			return Trip{}, ErrDriverBusy
		}
		return Trip{}, fmt.Errorf("не получилось добавить поездку в базу: %w", err)
	}

	query, args, err = sqlb.Insert("trip_status_history").
		Columns("trip_id", "to_status").
		Values(trip.ID, "active").
		ToSql()
	if err != nil {
		return Trip{}, err
	}
	_, err = r.save(ctx, query, args...)
	if err != nil {
		return Trip{}, fmt.Errorf("поездка создана, но статус в журнал не записался: %w", err)
	}

	return r.Get(ctx, trip.ID)
}

func (r *Repo) Get(ctx context.Context, id string) (Trip, error) {
	query, args, err := sqlb.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at", "created_at", "updated_at",
	).From("trips").Where("id = ?", id).ToSql()
	if err != nil {
		return Trip{}, err
	}

	var trip Trip
	err = r.load(ctx, query, args...).Scan(
		&trip.ID, &trip.UserID, &trip.DriverID,
		&trip.StartLatitude, &trip.StartLongitude, &trip.EndLatitude, &trip.EndLongitude,
		&trip.Price, &trip.Status, &trip.StartedAt, &trip.FinishedAt, &trip.CreatedAt, &trip.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Trip{}, ErrNotFound
	}
	if err != nil {
		return Trip{}, fmt.Errorf("не получилось прочитать поездку из базы: %w", err)
	}
	return trip, nil
}

func (r *Repo) Finish(ctx context.Context, id string) (Trip, error) {
	moment := time.Now()
	query, args, err := sqlb.Update("trips").
		Set("status", "completed").
		Set("finished_at", moment).
		Set("updated_at", moment).
		Where("id = ? AND status = ?", id, "active").
		ToSql()
	if err != nil {
		return Trip{}, err
	}

	tag, err := r.save(ctx, query, args...)
	if err != nil {
		return Trip{}, fmt.Errorf("не получилось закрыть поездку: %w", err)
	}
	if tag.RowsAffected() == 0 {
		found, readErr := r.Get(ctx, id)
		if readErr != nil {
			return Trip{}, readErr
		}
		if found.Status == "completed" {
			return Trip{}, ErrCompleted
		}
		return Trip{}, fmt.Errorf("поездка нашлась, но закрыть её не вышло")
	}

	query, args, err = sqlb.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status").
		Values(id, "active", "completed").
		ToSql()
	if err != nil {
		return Trip{}, err
	}
	_, err = r.save(ctx, query, args...)
	if err != nil {
		return Trip{}, fmt.Errorf("поездка закрыта, но запись в журнал не попала: %w", err)
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

package position

import (
	"context"
	"fmt"
	"time"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres/transaction"
)

func (r *Repo) List(ctx context.Context, tripID string) ([]domaintrip.Point, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := r.query(ctx, `
		SELECT id, trip_id, latitude, longitude, recorded_at
		FROM trip_positions
		WHERE trip_id = $1
		ORDER BY recorded_at, id`, tripID)
	if err != nil {
		return nil, fmt.Errorf("не получилось прочитать маршрут: %w", err)
	}
	defer rows.Close()

	out := make([]domaintrip.Point, 0)
	for rows.Next() {
		var item domaintrip.Point
		if err = rows.Scan(&item.ID, &item.TripID, &item.Latitude, &item.Longitude, &item.RecordedAt); err != nil {
			return nil, fmt.Errorf("не получилось прочитать точку маршрута: %w", err)
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("маршрут прочитался не до конца: %w", err)
	}
	return out, nil
}

func (r *Repo) Quiet(ctx context.Context, before time.Time) ([]domaintrip.Quiet, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := r.query(ctx, `
		SELECT id, driver_id, COALESCE(last_position_at, started_at)
		FROM trips
		WHERE status = $1
		  AND COALESCE(last_position_at, started_at) < $2
		ORDER BY COALESCE(last_position_at, started_at), id`,
		domaintrip.StatusActive, before)
	if err != nil {
		return nil, fmt.Errorf("не получилось выбрать поездки без свежих координат: %w", err)
	}
	defer rows.Close()

	out := make([]domaintrip.Quiet, 0)
	for rows.Next() {
		var item domaintrip.Quiet
		if err = rows.Scan(&item.ID, &item.DriverID, &item.Since); err != nil {
			return nil, fmt.Errorf("не получилось прочитать поездку для опроса: %w", err)
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("выборка поездок для опроса оборвалась: %w", err)
	}
	return out, nil
}

func (r *Repo) query(ctx context.Context, sql string, args ...any) (rows, error) {
	if tx, ok := transaction.Current(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	return r.pool.Query(ctx, sql, args...)
}

type rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

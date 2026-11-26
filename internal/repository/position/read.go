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

const (
	quietFilter = `
		SELECT id, driver_id, COALESCE(last_position_at, started_at)
		FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at) < $1::timestamptz - make_interval(secs => $2)
	`
	quietAfter = ` AND (COALESCE(last_position_at, started_at), id) > ($4, $5)`
	quietOrder = ` ORDER BY COALESCE(last_position_at, started_at), id LIMIT $3`
)

func (r *Repo) Quiet(ctx context.Context, at time.Time, staleAfter time.Duration, after domaintrip.Quiet, limit int) ([]domaintrip.Quiet, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := r.quietRows(ctx, at, staleAfter, after, limit)
	if err != nil {
		return nil, fmt.Errorf("не получилось выбрать поездки без свежих координат: %w", err)
	}
	defer rows.Close()

	out := make([]domaintrip.Quiet, 0, limit)
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

func (r *Repo) quietRows(ctx context.Context, at time.Time, staleAfter time.Duration, after domaintrip.Quiet, limit int) (rows, error) {
	if after.ID == "" {
		return r.query(ctx, quietFilter+quietOrder, at, staleAfter.Seconds(), limit)
	}
	return r.query(ctx, quietFilter+quietAfter+quietOrder, at, staleAfter.Seconds(), limit, after.Since, after.ID)
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

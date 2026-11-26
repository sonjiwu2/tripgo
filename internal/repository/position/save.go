package position

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

func (r *Repo) Save(ctx context.Context, point domaintrip.Point) (domaintrip.Point, error) {
	var status domaintrip.Status
	err := r.load(ctx, `SELECT status FROM trips WHERE id = $1`, point.TripID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Point{}, domaintrip.ErrTripNotFound
	}
	if err != nil {
		return domaintrip.Point{}, fmt.Errorf("не получилось прочитать поездку перед координатой: %w", err)
	}
	if status != domaintrip.StatusActive {
		return domaintrip.Point{}, domaintrip.ErrTripCompleted
	}

	err = r.load(ctx, `
		INSERT INTO trip_positions (trip_id, latitude, longitude, recorded_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		point.TripID, point.Latitude, point.Longitude, point.RecordedAt,
	).Scan(&point.ID)
	if err != nil {
		return domaintrip.Point{}, fmt.Errorf("не получилось записать координату: %w", err)
	}

	tag, err := r.save(ctx, `
		UPDATE trips
		SET last_position_at = GREATEST(last_position_at, $1),
		    updated_at = $2
		WHERE id = $3 AND status = $4`,
		point.RecordedAt, time.Now(), point.TripID, domaintrip.StatusActive,
	)
	if err != nil {
		return domaintrip.Point{}, fmt.Errorf("не получилось обновить время последней координаты: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domaintrip.Point{}, domaintrip.ErrTripCompleted
	}
	return point, nil
}

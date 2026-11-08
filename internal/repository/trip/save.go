package trip

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres/transaction"
)

func (r *Repo) Save(ctx context.Context, item domaintrip.Trip) error {
	cols := []string{
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status", "started_at",
	}
	vals := []any{
		item.ID, item.UserID, item.DriverID,
		item.StartLatitude, item.StartLongitude, item.EndLatitude, item.EndLongitude,
		item.Price, domaintrip.StatusActive, item.StartedAt,
	}

	query, args, err := sqlb.Insert("trips").Columns(cols...).Values(vals...).ToSql()
	if err != nil {
		return err
	}
	if _, err = r.save(ctx, query, args...); err != nil {
		if driverIsBusy(err) {
			return domaintrip.ErrDriverBusy
		}
		return fmt.Errorf("не получилось добавить поездку в базу: %w", err)
	}

	query, args, err = sqlb.Insert("trip_status_history").
		Columns("trip_id", "to_status").
		Values(item.ID, domaintrip.StatusActive).
		ToSql()
	if err != nil {
		return err
	}
	if _, err = r.save(ctx, query, args...); err != nil {
		return fmt.Errorf("поездка создана, но статус в журнал не записался: %w", err)
	}
	return nil
}

func (r *Repo) Complete(ctx context.Context, item domaintrip.Trip) error {
	query, args, err := sqlb.Update("trips").
		Set("status", item.Status).
		Set("finished_at", item.FinishedAt).
		Set("updated_at", item.UpdatedAt).
		Where("id = ? AND status = ?", item.ID, domaintrip.StatusActive).
		ToSql()
	if err != nil {
		return err
	}

	tag, err := r.save(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("не получилось закрыть поездку: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return r.missedFinish(ctx, item.ID)
	}

	query, args, err = sqlb.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status").
		Values(item.ID, domaintrip.StatusActive, domaintrip.StatusCompleted).
		ToSql()
	if err != nil {
		return err
	}
	if _, err = r.save(ctx, query, args...); err != nil {
		return fmt.Errorf("поездка закрыта, но запись в журнал не попала: %w", err)
	}
	return nil
}

func (r *Repo) missedFinish(ctx context.Context, id string) error {
	found, err := r.Find(ctx, id)
	if err != nil {
		return err
	}
	if found.Status == domaintrip.StatusCompleted {
		return domaintrip.ErrTripCompleted
	}
	return fmt.Errorf("поездка нашлась, но закрыть её не вышло")
}

func (r *Repo) save(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	if tx, ok := transaction.Current(ctx); ok {
		return tx.Exec(ctx, query, args...)
	}
	return r.pool.Exec(ctx, query, args...)
}

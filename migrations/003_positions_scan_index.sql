-- +goose NO TRANSACTION

-- +goose Up

CREATE INDEX CONCURRENTLY trips_active_quiet_cursor_idx
    ON trips ((COALESCE(last_position_at, started_at)), id)
    WHERE status = 'active';

DROP INDEX CONCURRENTLY IF EXISTS trips_active_quiet_idx;

-- +goose Down

CREATE INDEX CONCURRENTLY trips_active_quiet_idx
    ON trips ((COALESCE(last_position_at, started_at)))
    WHERE status = 'active';

DROP INDEX CONCURRENTLY IF EXISTS trips_active_quiet_cursor_idx;

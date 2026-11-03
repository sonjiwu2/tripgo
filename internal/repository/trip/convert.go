package trip

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func driverIsBusy(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_driver_idx"
}

package trip

import (
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func New(pool *pgxpool.Pool, queryTimeout time.Duration) *Repo {
	return &Repo{pool: pool, queryTimeout: queryTimeout}
}

var sqlb = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

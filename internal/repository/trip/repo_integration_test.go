//go:build integration

package trip_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/sonjiwu2/tripgo/internal/config"
	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres"
	"github.com/sonjiwu2/tripgo/internal/infra/postgres/transaction"
	"github.com/sonjiwu2/tripgo/internal/repository/trip"
)

func TestMain(m *testing.M) {
	os.Exit(migrateAndRun(m))
}

func migrateAndRun(m *testing.M) int {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		panic("не задана DATABASE_URL")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err = goose.SetDialect("postgres"); err != nil {
		panic(err)
	}
	if err = goose.Up(db, migrationsDir()); err != nil {
		panic(err)
	}
	return m.Run()
}

func TestSaveRollsBackBothTables(t *testing.T) {
	ctx := context.Background()
	pool := openPool(t)
	repo := trip.New(pool, 3*time.Second)
	tx := transaction.NewTxManager(pool)
	item := newTrip(t)
	t.Cleanup(func() { dropTrip(t, pool, item.ID) })

	err := tx.Do(ctx, func(ctx context.Context) error {
		if err := repo.Save(ctx, item); err != nil {
			return err
		}
		return errors.New("проверка отката")
	})
	if err == nil || err.Error() != "проверка отката" {
		t.Fatalf("err %v", err)
	}
	if n := countTrips(t, pool, item.ID); n != 0 {
		t.Fatalf("trips left %d", n)
	}
	if n := countHistory(t, pool, item.ID); n != 0 {
		t.Fatalf("history left %d", n)
	}
}

func TestSaveWritesTripAndHistory(t *testing.T) {
	ctx := context.Background()
	pool := openPool(t)
	repo := trip.New(pool, 3*time.Second)
	tx := transaction.NewTxManager(pool)
	item := newTrip(t)
	t.Cleanup(func() { dropTrip(t, pool, item.ID) })

	err := tx.Do(ctx, func(ctx context.Context) error {
		return repo.Save(ctx, item)
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := countTrips(t, pool, item.ID); n != 1 {
		t.Fatalf("trips %d", n)
	}
	if n := countHistory(t, pool, item.ID); n != 1 {
		t.Fatalf("history %d", n)
	}
	found, err := repo.Find(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Status != domaintrip.StatusActive || found.DriverID != item.DriverID {
		t.Fatalf("found %+v", found)
	}
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DB{
		URL:             os.Getenv("DATABASE_URL"),
		MaxConns:        4,
		MinConns:        1,
		MaxConnLifetime: time.Minute,
		ConnectTimeout:  5 * time.Second,
		QueryTimeout:    3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newTrip(t *testing.T) domaintrip.Trip {
	t.Helper()
	item, err := domaintrip.New(
		uuid.NewString(),
		uuid.NewString(),
		uuid.NewString(),
		59.93, 30.31, 59.95, 30.35,
		450,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func countTrips(t *testing.T, pool *pgxpool.Pool, id string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), "select count(*) from trips where id = $1", id).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func countHistory(t *testing.T, pool *pgxpool.Pool, id string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), "select count(*) from trip_status_history where trip_id = $1", id).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func dropTrip(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "delete from trip_status_history where trip_id = $1", id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), "delete from trips where id = $1", id)
	if err != nil {
		t.Fatal(err)
	}
}

func migrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "migrations"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
}

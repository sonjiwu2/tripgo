//go:build integration

package position_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	"github.com/sonjiwu2/tripgo/internal/repository/position"
	triprepo "github.com/sonjiwu2/tripgo/internal/repository/trip"
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

func TestSaveRollsBackPointAndStamp(t *testing.T) {
	ctx := context.Background()
	pool := openPool(t)
	trips := triprepo.New(pool, 3*time.Second)
	points := position.New(pool, 3*time.Second)
	tx := transaction.NewTxManager(pool, 3*time.Second)
	item := newTrip(t, time.Now().Add(-2*time.Hour))
	t.Cleanup(func() { dropTrip(t, pool, item.ID) })

	if err := tx.Do(ctx, func(ctx context.Context) error {
		return trips.Save(ctx, item)
	}); err != nil {
		t.Fatal(err)
	}

	point, err := domaintrip.NewPoint(item.ID, 59.93, 30.33, time.Now().Add(-time.Minute), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	err = tx.Do(ctx, func(ctx context.Context) error {
		if _, err := points.Save(ctx, point); err != nil {
			return err
		}
		return errors.New("проверка отката")
	})
	if err == nil || err.Error() != "проверка отката" {
		t.Fatalf("err %v", err)
	}
	if n := countPoints(t, pool, item.ID); n != 0 {
		t.Fatalf("points left %d", n)
	}
	found, err := trips.Find(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.LastPositionAt != nil {
		t.Fatalf("stamp %v", found.LastPositionAt)
	}
}

func TestOldPointDoesNotAgeTrip(t *testing.T) {
	ctx := context.Background()
	pool := openPool(t)
	trips := triprepo.New(pool, 3*time.Second)
	points := position.New(pool, 3*time.Second)
	tx := transaction.NewTxManager(pool, 3*time.Second)
	item := newTrip(t, time.Now().Add(-3*time.Hour))
	t.Cleanup(func() { dropTrip(t, pool, item.ID) })
	if err := tx.Do(ctx, func(ctx context.Context) error {
		return trips.Save(ctx, item)
	}); err != nil {
		t.Fatal(err)
	}

	fresh := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Microsecond)
	old := fresh.Add(-time.Hour)
	if err := tx.Do(ctx, func(ctx context.Context) error {
		point, err := domaintrip.NewPoint(item.ID, 59.93, 30.33, fresh, time.Now())
		if err != nil {
			return err
		}
		_, err = points.Save(ctx, point)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Do(ctx, func(ctx context.Context) error {
		point, err := domaintrip.NewPoint(item.ID, 59.94, 30.34, old, time.Now())
		if err != nil {
			return err
		}
		_, err = points.Save(ctx, point)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	found, err := trips.Find(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.LastPositionAt == nil || !found.LastPositionAt.Equal(fresh) {
		t.Fatalf("last %v want %s", found.LastPositionAt, fresh)
	}
	list, err := points.List(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("points %d", len(list))
	}
	if !list[0].RecordedAt.Equal(old) || !list[1].RecordedAt.Equal(fresh) {
		t.Fatalf("order %s then %s", list[0].RecordedAt, list[1].RecordedAt)
	}
}

func TestQuietSkipsFreshAndCompleted(t *testing.T) {
	ctx := context.Background()
	pool := openPool(t)
	trips := triprepo.New(pool, 3*time.Second)
	points := position.New(pool, 3*time.Second)
	tx := transaction.NewTxManager(pool, 3*time.Second)

	stale := newTrip(t, time.Now().Add(-2*time.Hour))
	fresh := newTrip(t, time.Now().Add(-2*time.Hour))
	done := newTrip(t, time.Now().Add(-2*time.Hour))
	justStarted := newTrip(t, time.Now().Add(-10*time.Second))
	for _, item := range []domaintrip.Trip{stale, fresh, done, justStarted} {
		item := item
		t.Cleanup(func() { dropTrip(t, pool, item.ID) })
		if err := tx.Do(ctx, func(ctx context.Context) error {
			return trips.Save(ctx, item)
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := tx.Do(ctx, func(ctx context.Context) error {
		point, err := domaintrip.NewPoint(fresh.ID, 59.9, 30.3, time.Now().Add(-5*time.Second), time.Now())
		if err != nil {
			return err
		}
		_, err = points.Save(ctx, point)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Do(ctx, func(ctx context.Context) error {
		found, err := trips.Find(ctx, done.ID)
		if err != nil {
			return err
		}
		if err = found.Finish(time.Now()); err != nil {
			return err
		}
		return trips.Complete(ctx, found)
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := points.Quiet(ctx, time.Now(), time.Minute, domaintrip.Quiet{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, row := range rows {
		got[row.ID] = true
	}
	if !got[stale.ID] {
		t.Fatal("stale trip missing")
	}
	if got[fresh.ID] || got[done.ID] || got[justStarted.ID] {
		t.Fatalf("extra rows %v", got)
	}
}

func TestQuietPagesWithEqualTimes(t *testing.T) {
	ctx := context.Background()
	pool := openPool(t)
	trips := triprepo.New(pool, 3*time.Second)
	points := position.New(pool, 3*time.Second)
	tx := transaction.NewTxManager(pool, 3*time.Second)
	at := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

	want := make([]string, 0, 5)
	for range 5 {
		item := newTrip(t, at)
		t.Cleanup(func() { dropTrip(t, pool, item.ID) })
		if err := tx.Do(ctx, func(ctx context.Context) error {
			return trips.Save(ctx, item)
		}); err != nil {
			t.Fatal(err)
		}
		want = append(want, item.ID)
	}
	slices.Sort(want)

	const batch = 2
	after := domaintrip.Quiet{ID: uuid.Nil.String(), Since: at.Add(-time.Microsecond)}
	got := make([]string, 0, len(want))
	for page := range 3 {
		rows, err := points.Quiet(ctx, at.Add(2*time.Minute), time.Minute, after, batch)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != min(batch, len(want)-page*batch) {
			t.Fatalf("page %d: rows %d", page, len(rows))
		}
		for _, row := range rows {
			got = append(got, row.ID)
		}
		after = rows[len(rows)-1]
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ids %v, want %v", got, want)
	}
}

func newTrip(t *testing.T, at time.Time) domaintrip.Trip {
	t.Helper()
	item, err := domaintrip.New(
		uuid.NewString(),
		uuid.NewString(),
		uuid.NewString(),
		59.93, 30.31, 59.95, 30.35,
		450,
		at,
	)
	if err != nil {
		t.Fatal(err)
	}
	return item
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

func countPoints(t *testing.T, pool *pgxpool.Pool, tripID string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), "select count(*) from trip_positions where trip_id = $1", tripID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func dropTrip(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "delete from trips where id = $1", id); err != nil {
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

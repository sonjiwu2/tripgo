package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxFunc func(ctx context.Context) error

type TxManager interface {
	Do(ctx context.Context, fn TxFunc) error
}

type manager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &manager{pool: pool}
}

type txKey struct{}

func (m *manager) Do(ctx context.Context, fn TxFunc) error {
	if _, ok := GetTransaction(ctx); ok {
		return fn(ctx)
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	err = fn(context.WithValue(ctx, txKey{}, tx))
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("не удалось зафиксировать транзакцию: %w", err)
	}
	committed = true
	return nil
}

func GetTransaction(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

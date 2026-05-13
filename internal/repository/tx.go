package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type txContextKey struct{}

type TxRunner struct {
	db interface {
		Begin(context.Context) (pgx.Tx, error)
	}
}

func NewTxRunner(db interface {
	Begin(context.Context) (pgx.Tx, error)
}) *TxRunner {
	return &TxRunner{db: db}
}

func (r *TxRunner) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("txRunner.RunInTx begin: %w", err)
	}
	defer tx.Rollback(ctx)

	txCtx := context.WithValue(ctx, txContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("txRunner.RunInTx commit: %w", err)
	}
	return nil
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx, ok
}

func queriesFor(ctx context.Context, q *sqlc.Queries) *sqlc.Queries {
	tx, ok := txFromContext(ctx)
	if !ok {
		return q
	}
	return q.WithTx(tx)
}

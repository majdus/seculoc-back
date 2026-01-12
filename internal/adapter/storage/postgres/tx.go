package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TxManagerImpl implements the TxManager interface using pgx.
type TxManagerImpl struct {
	db *pgxpool.Pool
}

func NewTxManager(db *pgxpool.Pool) *TxManagerImpl {
	return &TxManagerImpl{db: db}
}

// WithTx executes the given function within a transaction.
func (tm *TxManagerImpl) WithTx(ctx context.Context, fn func(Querier) error) error {
	tx, err := tm.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Helper to ensure rollback on panic or error
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(ctx)
			panic(p) // Re-panic after rollback
		} else if err != nil {
			tx.Rollback(ctx) // err is non-nil if fn returns error
		} else {
			err = tx.Commit(ctx) // Commit if no error
		}
	}()

	q := New(tx) // Create Queries bound to the transaction
	err = fn(q)

	return err
}

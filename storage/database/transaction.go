package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type txContextKey struct{}

// TxFunc is the signature of functions executed within a managed database transaction.
type TxFunc func(ctx context.Context, tx *sql.Tx) error

// WithTx returns a new context carrying the given *sql.Tx.
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext retrieves the active *sql.Tx from context if one was injected by RunInTx or WithTx.
// Returns nil if no transaction is found in the context.
func TxFromContext(ctx context.Context) *sql.Tx {
	if ctx == nil {
		return nil
	}
	if tx, ok := ctx.Value(txContextKey{}).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// RunInTx executes fn inside a managed database transaction with default options.
//
// Key behaviors:
//   - If fn returns nil, the transaction is committed.
//   - If fn returns an error, the transaction is rolled back and the error is returned (joined with any rollback error).
//   - If fn panics, the transaction is rolled back and the panic is re-thrown.
//   - If a transaction is already active in ctx (nested RunInTx), fn participates in the existing transaction
//     without creating a nested transaction or committing prematurely.
func RunInTx(ctx context.Context, db Transactor, fn TxFunc) error {
	return RunInTxWithOptions(ctx, db, nil, fn)
}

// RunInTxWithOptions executes fn inside a managed database transaction with the provided sql.TxOptions.
// If opts is nil, driver defaults are used.
func RunInTxWithOptions(ctx context.Context, db Transactor, opts *sql.TxOptions, fn TxFunc) (err error) {
	if db == nil {
		return errors.New("database: transactor is nil")
	}
	if fn == nil {
		return errors.New("database: tx function is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Nested transaction support: if a transaction is already active in this context,
	// participate in the outer transaction without opening a new one.
	if existingTx := TxFromContext(ctx); existingTx != nil {
		return fn(ctx, existingTx)
	}

	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	txCtx := WithTx(ctx, tx)
	if err = fn(txCtx, tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return fmt.Errorf("%w (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

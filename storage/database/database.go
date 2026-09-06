package database

import (
	"context"
	"database/sql"
)

// Transactor is the contract for starting database transactions.
// Both Database and standard *sql.DB satisfy this interface.
type Transactor interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Database is the main contract that every implementation must satisfy
// (Oracle, PostgreSQL, MySQL, etc). This way, business code only depends
// on this interface, not on a specific driver.
type Database interface {
	Transactor

	// Connect opens a connection to the database according to the given Config.
	Connect(ctx context.Context) error

	// Close closes the connection.
	Close() error

	// Ping verifies the connection is still alive.
	Ping(ctx context.Context) error

	// Exec runs a query that doesn't return rows (INSERT/UPDATE/DELETE/DDL).
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)

	// Query runs a query that returns multiple rows.
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// QueryRow runs a query that returns a single row.
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row

	// RunInTx runs fn inside a managed transaction with the given options (or nil for default).
	// Automatically commits on nil error, and rolls back on error or panic.
	RunInTx(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context, tx *sql.Tx) error) error

	// Driver returns the driver name, useful for logging/debugging.
	Driver() string

	// Stats returns connection pool statistics.
	Stats() sql.DBStats
}

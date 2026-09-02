package database

import (
	"context"
	"database/sql"
)

// Database is the main contract that every implementation must satisfy
// (Oracle, PostgreSQL, MySQL, etc). This way, business code only depends
// on this interface, not on a specific driver.
type Database interface {
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

	// BeginTx starts a transaction.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// Driver returns the driver name, useful for logging/debugging.
	Driver() string

	// Stats returns connection pool statistics.
	Stats() sql.DBStats
}

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/database"
	_ "github.com/go-sql-driver/mysql" // mysql driver
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Ensure Database implements database.Database interface
var _ database.Database = (*DB)(nil)

// Ensure DB implements health.Check interface
var _ health.Check = (*DB)(nil)

// DB MySQLDB is the Database implementation for MySQL/MariaDB.
type DB struct {
	cfg     Config
	db      *sql.DB
	logger  logger.Logger
	appName string
}

// NewMySQLDB creates a MySQL Database instance.
// Call Connect(ctx) to establish the connection pool, or manage lifecycle via your application container.
// If health.Registry is provided, it automatically registers a readiness check for MySQL.
func NewMySQLDB(
	l logger.Logger,
	cfg Config,
	hrs ...health.Registry,
) (*DB, error) {
	appName := coreContext.AppName(context.Background())

	db := &DB{
		cfg:     cfg,
		logger:  l,
		appName: appName,
	}

	for _, hr := range hrs {
		if hr != nil {
			hr.RegisterReadiness("mysql", db)
			break
		}
	}

	return db, nil
}

func (m *DB) Connect(ctx context.Context) error {
	dsn := m.cfg.DSN
	if dsn == "" {
		// format: user:password@tcp(host:port)/dbname?parseTime=true
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
			m.cfg.User, m.cfg.Password, m.cfg.Host, m.cfg.Port, m.cfg.DBName,
		)
	}

	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("mysql: failed to open connection: %w", err)
	}

	applyPoolSettings(sqlDB, m.cfg)

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("mysql: ping failed: %w", err)
	}

	m.db = sqlDB
	return nil
}

func (m *DB) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}

func (m *DB) Ping(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

func (m *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	ctx, span := m.startSpan(ctx, "Exec", query, args...)
	defer span.End()

	res, err := m.db.ExecContext(ctx, query, args...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	m.log(ctx, "EXEC", query, start, err)

	return res, err
}

func (m *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	ctx, span := m.startSpan(ctx, "Query", query, args...)
	defer span.End()

	rows, err := m.db.QueryContext(ctx, query, args...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	m.log(ctx, "QUERY", query, start, err)

	return rows, err
}

func (m *DB) QueryRow(ctx context.Context, query string, args ...any) database.Row {
	start := time.Now()
	ctx, span := m.startSpan(ctx, "QueryRow", query, args...)

	rows, err := m.db.QueryContext(ctx, query, args...)

	return &tracedRow{
		rows:  rows,
		err:   err,
		span:  span,
		start: start,
		logFn: func(s time.Time, err error) {
			m.log(ctx, "QUERYROW", query, s, err)
		},
	}
}

type tracedRow struct {
	rows  *sql.Rows
	err   error
	span  trace.Span
	start time.Time
	logFn func(start time.Time, err error)
	done  bool
}

func (r *tracedRow) Scan(dest ...any) error {
	if r.done {
		return r.err
	}
	r.done = true

	err := r.scan(dest...)
	r.err = err

	if r.span != nil {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			r.span.RecordError(err)
			r.span.SetStatus(codes.Error, err.Error())
		}
		r.span.End()
	}

	if r.logFn != nil {
		r.logFn(r.start, err)
	}

	return err
}

func (r *tracedRow) scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	defer r.rows.Close()
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	err := r.rows.Scan(dest...)
	if err != nil {
		return err
	}
	return r.rows.Close()
}

func (r *tracedRow) Err() error {
	return r.err
}

func (m *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	start := time.Now()
	ctx, span := m.startSpan(ctx, "BeginTx", "BEGIN", nil)
	defer span.End()

	tx, err := m.db.BeginTx(ctx, opts)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	m.log(ctx, "BEGINTX", "BEGIN", start, err)

	return tx, err
}

// RunInTx executes fn inside a managed database transaction with the provided sql.TxOptions.
// Automatically commits on nil error, and rolls back on error or panic.
func (m *DB) RunInTx(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context, tx *sql.Tx) error) error {
	return database.RunInTxWithOptions(ctx, m, opts, fn)
}

func (m *DB) startSpan(ctx context.Context, operation string, query string, args ...any) (context.Context, trace.Span) {
	if !m.cfg.Tracing.Enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	tracer := otel.Tracer("mysql-client")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("mysql: %s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "mysql"),
			attribute.String("db.operation", operation),
			attribute.String("db.statement", query),
			attribute.String("db.user", m.cfg.User),
			attribute.String("db.name", m.cfg.DBName),
			attribute.String("server.address", m.cfg.Host),
			attribute.Int("server.port", m.cfg.Port),
		),
	)
	return ctx, span
}

func (m *DB) log(ctx context.Context, method string, query string, start time.Time, err error) {
	if !m.cfg.Logger.Enabled {
		return
	}

	statusCode := 200
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			statusCode = 404
		} else {
			statusCode = 500
		}
	}

	payload := logger.LogPayload{
		Timestamp: start,
		AppName:   m.appName,
		Type:      "STORAGE_MYSQL",
		Method:    method,
		IpAddress: fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port),
		Status:    statusCode,
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      m.cfg.DBName,
		Request:   query,
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		payload.Error = err.Error()
		m.logger.Error(ctx, fmt.Sprintf("MYSQL %s", method), payload)
	} else {
		if errors.Is(err, sql.ErrNoRows) {
			payload.Response = "null"
		}
		m.logger.Info(ctx, fmt.Sprintf("MYSQL %s", method), payload)
	}
}

func (m *DB) Driver() string {
	return "mysql"
}

func (m *DB) Stats() sql.DBStats {
	return m.db.Stats()
}

// Check implements health.Check for MySQL readiness.
func (m *DB) Check(ctx context.Context) health.Health {
	if m.db == nil {
		return health.DownWithDetails(map[string]any{"error": "database is not connected"})
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := m.db.PingContext(ctx); err != nil {
		return health.DownWithDetails(map[string]any{"error": err.Error()})
	}
	return health.Up()
}

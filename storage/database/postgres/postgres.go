package postgres

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
	_ "github.com/lib/pq" // postgres driver
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Ensure Database implements database.Database interface
var _ database.Database = (*DB)(nil)

// Ensure DB implements health.Check interface
var _ health.Check = (*DB)(nil)

// DB PostgresDB is the Database implementation for PostgreSQL.
type DB struct {
	cfg     Config
	db      *sql.DB
	logger  logger.Logger
	appName string
}

// NewPostgresDB creates a PostgreSQL Database instance.
// Call Connect(ctx) to establish the connection pool, or manage lifecycle via your application container.
// If health.Registry is provided, it automatically registers a readiness check for this database.
func NewPostgresDB(
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
			hr.RegisterReadiness("postgres", db)
			break
		}
	}

	return db, nil
}

func (p *DB) Connect(ctx context.Context) error {
	dsn := p.cfg.DSN
	if dsn == "" {
		sslMode := p.cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			p.cfg.Host, p.cfg.Port, p.cfg.User, p.cfg.Password, p.cfg.DBName, sslMode,
		)
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("postgres: failed to open connection: %w", err)
	}

	applyPoolSettings(sqlDB, p.cfg)

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres: ping failed: %w", err)
	}

	p.db = sqlDB
	return nil
}

func (p *DB) Close() error {
	if p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *DB) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func (p *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	ctx, span := p.startSpan(ctx, "Exec", query, args...)
	defer span.End()

	res, err := p.db.ExecContext(ctx, query, args...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	p.log(ctx, "EXEC", query, start, err)

	return res, err
}

func (p *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	ctx, span := p.startSpan(ctx, "Query", query, args...)
	defer span.End()

	rows, err := p.db.QueryContext(ctx, query, args...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	p.log(ctx, "QUERY", query, start, err)

	return rows, err
}

func (p *DB) QueryRow(ctx context.Context, query string, args ...any) database.Row {
	start := time.Now()
	ctx, span := p.startSpan(ctx, "QueryRow", query, args...)

	rows, err := p.db.QueryContext(ctx, query, args...)

	return &tracedRow{
		rows:  rows,
		err:   err,
		span:  span,
		start: start,
		logFn: func(s time.Time, err error) {
			p.log(ctx, "QUERYROW", query, s, err)
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

func (p *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	start := time.Now()
	ctx, span := p.startSpan(ctx, "BeginTx", "BEGIN", nil)
	defer span.End()

	tx, err := p.db.BeginTx(ctx, opts)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	p.log(ctx, "BEGINTX", "BEGIN", start, err)

	return tx, err
}

// RunInTx executes fn inside a managed database transaction with the provided sql.TxOptions.
// Automatically commits on nil error, and rolls back on error or panic.
func (p *DB) RunInTx(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context, tx *sql.Tx) error) error {
	return database.RunInTxWithOptions(ctx, p, opts, fn)
}

func (p *DB) startSpan(ctx context.Context, operation string, query string, args ...any) (context.Context, trace.Span) {
	if !p.cfg.Tracing.Enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	tracer := otel.Tracer("postgres-client")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("postgres: %s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", operation),
			attribute.String("db.statement", query),
			attribute.String("db.user", p.cfg.User),
			attribute.String("db.name", p.cfg.DBName),
			attribute.String("server.address", p.cfg.Host),
			attribute.Int("server.port", p.cfg.Port),
		),
	)
	return ctx, span
}

func (p *DB) log(ctx context.Context, method string, query string, start time.Time, err error) {
	if !p.cfg.Logger.Enabled {
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
		AppName:   p.appName,
		Type:      "STORAGE_POSTGRES",
		Method:    method,
		IpAddress: fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port),
		Status:    statusCode,
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      p.cfg.DBName,
		Request:   query,
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		payload.Error = err.Error()
		p.logger.Error(ctx, fmt.Sprintf("POSTGRES %s", method), payload)
	} else {
		if errors.Is(err, sql.ErrNoRows) {
			payload.Response = "null"
		}
		p.logger.Info(ctx, fmt.Sprintf("POSTGRES %s", method), payload)
	}
}

func (p *DB) Driver() string {
	return "postgres"
}

func (p *DB) Stats() sql.DBStats {
	return p.db.Stats()
}

// Check implements health.Check for PostgreSQL readiness.
func (p *DB) Check(ctx context.Context) health.Health {
	if p.db == nil {
		return health.DownWithDetails(map[string]any{"error": "database is not connected"})
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := p.db.PingContext(ctx); err != nil {
		return health.DownWithDetails(map[string]any{"error": err.Error()})
	}
	return health.Up()
}

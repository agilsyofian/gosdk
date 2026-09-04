package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/storage/database"
	_ "github.com/lib/pq" // postgres driver
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

// Ensure Database implements xdatabase.Database interface
var _ database.Database = (*DB)(nil)

// DB PostgresDB is the Database implementation for PostgreSQL.
type DB struct {
	cfg     Config
	db      *sql.DB
	logger  logger.Logger
	appName string
}

// NewPostgresDB creates an empty instance; call Connect afterward.
func NewPostgresDB(
	lc fx.Lifecycle,
	l logger.Logger,
	cfg Config,
) (*DB, error) {

	appName := coreContext.AppName(context.Background())

	db := &DB{
		cfg:     cfg,
		logger:  l,
		appName: appName,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := db.Connect(ctx); err != nil {
				l.Error(ctx, fmt.Sprintf("failed to connect database: %s", err.Error()), logger.LogPayload{
					Type: "DATABASE",
				})
				return err
			}

			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := db.Ping(pingCtx); err != nil {
				db.Close()
				l.Error(ctx, fmt.Sprintf("failed to ping database: %s", err.Error()), logger.LogPayload{
					Type: "DATABASE",
				})
				return fmt.Errorf("ping database: %w", err)
			}

			l.Info(ctx, "database connection established", logger.LogPayload{
				Type:   "DATABASE",
				Method: "START",
			})
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info(ctx, "stopping database", logger.LogPayload{
				Type:   "DATABASE",
				Method: "SHUTDOWN",
			})
			return db.Close()
		},
	})

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

func (p *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	ctx, span := p.startSpan(ctx, "QueryRow", query, args...)
	defer span.End()

	row := p.db.QueryRowContext(ctx, query, args...)

	// sql.Row doesn't return error until Scan, so we can only log the start
	p.log(ctx, "QUERYROW", query, start, nil)

	return row
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
		),
	)
	return ctx, span
}

func (p *DB) log(ctx context.Context, method string, query string, start time.Time, err error) {
	if !p.cfg.Logger.Enabled {
		return
	}

	payload := logger.LogPayload{
		Timestamp: start,
		AppName:   p.appName,
		Type:      "STORAGE_POSTGRES",
		Method:    method,
		IpAddress: fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port),
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      p.cfg.DBName,
		Request:   query,
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil {
		payload.Error = err.Error()
		p.logger.Error(ctx, fmt.Sprintf("POSTGRES %s", method), payload)
	} else {
		p.logger.Info(ctx, fmt.Sprintf("POSTGRES %s", method), payload)
	}
}

func (p *DB) Driver() string {
	return "postgres"
}

func (p *DB) Stats() sql.DBStats {
	return p.db.Stats()
}

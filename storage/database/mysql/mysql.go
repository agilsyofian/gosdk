package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/storage/database"
	_ "github.com/go-sql-driver/mysql" // mysql driver
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

// Ensure Database implements xdatabase.Database interface
var _ database.Database = (*DB)(nil)

// DB MySQLDB is the Database implementation for MySQL/MariaDB.
type DB struct {
	cfg     Config
	db      *sql.DB
	logger  logger.Logger
	appName string
}

// NewMySQLDB creates an empty instance; call Connect afterward.
func NewMySQLDB(
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

func (m *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	ctx, span := m.startSpan(ctx, "QueryRow", query, args...)
	defer span.End()

	row := m.db.QueryRowContext(ctx, query, args...)

	// sql.Row doesn't return error until Scan, so we can only log the start
	m.log(ctx, "QUERYROW", query, start, nil)

	return row
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
		),
	)
	return ctx, span
}

func (m *DB) log(ctx context.Context, method string, query string, start time.Time, err error) {
	if !m.cfg.Logger.Enabled {
		return
	}

	payload := logger.LogPayload{
		Timestamp: start,
		AppName:   m.appName,
		Type:      "STORAGE_MYSQL",
		Method:    method,
		IpAddress: fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port),
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      m.cfg.DBName,
		Request:   query,
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil {
		payload.Error = err.Error()
		m.logger.Error(ctx, fmt.Sprintf("MYSQL %s", method), payload)
	} else {
		m.logger.Info(ctx, fmt.Sprintf("MYSQL %s", method), payload)
	}
}

func (m *DB) Driver() string {
	return "mysql"
}

func (m *DB) Stats() sql.DBStats {
	return m.db.Stats()
}

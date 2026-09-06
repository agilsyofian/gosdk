package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/database"
	_ "github.com/sijms/go-ora/v2" // pure-Go Oracle driver (no Oracle Instant Client needed)
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

// Ensure Database implements database.Database interface
var _ database.Database = (*DB)(nil)

// Ensure DB implements health.Check interface
var _ health.Check = (*DB)(nil)

// DB OracleDB is the Database implementation for Oracle.
// Uses the go-ora driver because it's pure Go (no OCI/Instant Client required).
// If the project already uses godror + Instant Client, just swap the driver
// name & DSN builder below without changing the public interface.
type DB struct {
	cfg     Config
	db      *sql.DB
	logger  logger.Logger
	appName string
}

// NewOracleDB creates an empty instance; call Connect afterward.
// If health.Registry is provided, it automatically registers a readiness check for Oracle.
func NewOracleDB(
	lc fx.Lifecycle,
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
			hr.RegisterReadiness("oracle", db)
			break
		}
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

func (o *DB) Connect(ctx context.Context) error {
	dsn := o.cfg.DSN
	if dsn == "" {
		service := o.cfg.ServiceName
		if service == "" {
			service = o.cfg.SID
		}
		// go-ora format: oracle://user:password@host:port/service_name
		dsn = fmt.Sprintf(
			"oracle://%s:%s@%s:%d/%s",
			o.cfg.User, o.cfg.Password, o.cfg.Host, o.cfg.Port, service,
		)
	}

	sqlDB, err := sql.Open("oracle", dsn)
	if err != nil {
		return fmt.Errorf("oracle: failed to open connection: %w", err)
	}

	applyPoolSettings(sqlDB, o.cfg)

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("oracle: ping failed: %w", err)
	}

	o.db = sqlDB
	return nil
}

func (o *DB) Close() error {
	if o.db == nil {
		return nil
	}
	return o.db.Close()
}

func (o *DB) Ping(ctx context.Context) error {
	return o.db.PingContext(ctx)
}

func (o *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	ctx, span := o.startSpan(ctx, "Exec", query, args...)
	defer span.End()

	res, err := o.db.ExecContext(ctx, query, args...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	o.log(ctx, "EXEC", query, start, err)

	return res, err
}

func (o *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	ctx, span := o.startSpan(ctx, "Query", query, args...)
	defer span.End()

	rows, err := o.db.QueryContext(ctx, query, args...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	o.log(ctx, "QUERY", query, start, err)

	return rows, err
}

func (o *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	ctx, span := o.startSpan(ctx, "QueryRow", query, args...)
	defer span.End()

	row := o.db.QueryRowContext(ctx, query, args...)

	// sql.Row doesn't return error until Scan, so we can only log the start
	o.log(ctx, "QUERYROW", query, start, nil)

	return row
}

func (o *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	start := time.Now()
	ctx, span := o.startSpan(ctx, "BeginTx", "BEGIN", nil)
	defer span.End()

	tx, err := o.db.BeginTx(ctx, opts)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	o.log(ctx, "BEGINTX", "BEGIN", start, err)

	return tx, err
}

// RunInTx executes fn inside a managed database transaction with the provided sql.TxOptions.
// Automatically commits on nil error, and rolls back on error or panic.
func (o *DB) RunInTx(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context, tx *sql.Tx) error) error {
	return database.RunInTxWithOptions(ctx, o, opts, fn)
}

func (o *DB) startSpan(ctx context.Context, operation string, query string, args ...any) (context.Context, trace.Span) {
	if !o.cfg.Tracing.Enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	tracer := otel.Tracer("oracle-client")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("oracle: %s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "oracle"),
			attribute.String("db.operation", operation),
			attribute.String("db.statement", query),
			attribute.String("db.user", o.cfg.User),
			attribute.String("db.name", o.cfg.DBName),
		),
	)
	return ctx, span
}

func (o *DB) log(ctx context.Context, method string, query string, start time.Time, err error) {
	if !o.cfg.Logger.Enabled {
		return
	}

	payload := logger.LogPayload{
		Timestamp: start,
		AppName:   o.appName,
		Type:      "STORAGE_ORACLE",
		Method:    method,
		IpAddress: fmt.Sprintf("%s:%d", o.cfg.Host, o.cfg.Port),
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      o.cfg.DBName,
		Request:   query,
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil {
		payload.Error = err.Error()
		o.logger.Error(ctx, fmt.Sprintf("ORACLE %s", method), payload)
	} else {
		o.logger.Info(ctx, fmt.Sprintf("ORACLE %s", method), payload)
	}
}

func (o *DB) Driver() string {
	return "oracle"
}

func (o *DB) Stats() sql.DBStats {
	return o.db.Stats()
}

// Check implements health.Check for Oracle readiness.
func (o *DB) Check(ctx context.Context) health.Health {
	if o.db == nil {
		return health.DownWithDetails(map[string]any{"error": "database is not connected"})
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := o.db.PingContext(ctx); err != nil {
		return health.DownWithDetails(map[string]any{"error": err.Error()})
	}
	return health.Up()
}

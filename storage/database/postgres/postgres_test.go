package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type mockLogger struct {
	mu        sync.Mutex
	infoLogs  []logEntry
	errorLogs []logEntry
}

type logEntry struct {
	msg     string
	payload logger.LogPayload
}

func (m *mockLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {}
func (m *mockLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (m *mockLogger) Fatal(ctx context.Context, msg string, payload logger.LogPayload) {}

func (m *mockLogger) Info(ctx context.Context, msg string, payload logger.LogPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoLogs = append(m.infoLogs, logEntry{msg: msg, payload: payload})
}

func (m *mockLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorLogs = append(m.errorLogs, logEntry{msg: msg, payload: payload})
}

var (
	testDriverOnce sync.Once
	testDriverInst = &mockDriver{}
)

type mockDriver struct {
	mu       sync.Mutex
	queryErr error
	rowsCols []string
	rowsVals [][]driver.Value
	execErr  error
}

func (d *mockDriver) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.queryErr = nil
	d.rowsCols = nil
	d.rowsVals = nil
	d.execErr = nil
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{d: d}, nil
}

type mockConn struct {
	d *mockDriver
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *mockConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	if c.d.queryErr != nil {
		return nil, c.d.queryErr
	}
	return &mockRows{
		cols: c.d.rowsCols,
		vals: c.d.rowsVals,
		pos:  0,
	}, nil
}

func (c *mockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	if c.d.execErr != nil {
		return nil, c.d.execErr
	}
	return driver.RowsAffected(1), nil
}

type mockRows struct {
	cols []string
	vals [][]driver.Value
	pos  int
}

func (r *mockRows) Columns() []string {
	return r.cols
}

func (r *mockRows) Close() error {
	return nil
}

func (r *mockRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.vals) {
		return io.EOF
	}
	row := r.vals[r.pos]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		}
	}
	r.pos++
	return nil
}

func setupTestDB(t *testing.T) (*DB, *mockLogger, *tracetest.SpanRecorder) {
	t.Helper()
	testDriverOnce.Do(func() {
		sql.Register("mock_postgres", testDriverInst)
	})
	testDriverInst.reset()

	sqlDB, err := sql.Open("mock_postgres", "")
	require.NoError(t, err)

	spanRecorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(tp)

	mockLog := &mockLogger{}
	cfg := Config{
		Host:   "localhost",
		Port:   5432,
		DBName: "test_db",
		User:   "postgres",
	}
	cfg.Tracing.Enabled = true
	cfg.Logger.Enabled = true

	db := &DB{
		cfg:     cfg,
		db:      sqlDB,
		logger:  mockLog,
		appName: "test-app",
	}

	return db, mockLog, spanRecorder
}

func TestDB_QueryRow_Success(t *testing.T) {
	db, mockLog, spanRecorder := setupTestDB(t)
	defer db.Close()

	testDriverInst.rowsCols = []string{"name", "age"}
	testDriverInst.rowsVals = [][]driver.Value{{"Alice", int64(30)}}

	ctx := context.Background()
	row := db.QueryRow(ctx, "SELECT name, age FROM users WHERE id = $1", 1)

	// Before Scan: log and span are not closed yet
	assert.Empty(t, mockLog.infoLogs)
	assert.Empty(t, spanRecorder.Ended())

	var name string
	var age int64
	err := row.Scan(&name, &age)
	require.NoError(t, err)
	assert.Equal(t, "Alice", name)
	assert.Equal(t, int64(30), age)

	// After Scan: span must be ended and log written with status 200
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "postgres: QueryRow", spans[0].Name())

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()
	require.Len(t, mockLog.infoLogs, 1)
	assert.Equal(t, 200, mockLog.infoLogs[0].payload.Status)
	assert.Equal(t, "STORAGE_POSTGRES", mockLog.infoLogs[0].payload.Type)
	assert.Equal(t, "QUERYROW", mockLog.infoLogs[0].payload.Method)
}

func TestDB_QueryRow_NoRows(t *testing.T) {
	db, mockLog, spanRecorder := setupTestDB(t)
	defer db.Close()

	testDriverInst.rowsCols = []string{"name"}
	testDriverInst.rowsVals = [][]driver.Value{} // Empty

	ctx := context.Background()
	row := db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", 999)

	var name string
	err := row.Scan(&name)
	require.ErrorIs(t, err, sql.ErrNoRows)

	// Span should be ended without error status
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()
	// Should be logged at Info level with status 404 and response "null"
	require.Len(t, mockLog.infoLogs, 1)
	assert.Equal(t, 404, mockLog.infoLogs[0].payload.Status)
	assert.Equal(t, "null", mockLog.infoLogs[0].payload.Response)
	assert.Empty(t, mockLog.errorLogs)
}

func TestDB_QueryRow_QueryError(t *testing.T) {
	db, mockLog, spanRecorder := setupTestDB(t)
	defer db.Close()

	testDriverInst.queryErr = errors.New("connection reset by peer")

	ctx := context.Background()
	row := db.QueryRow(ctx, "SELECT 1")

	var v int
	err := row.Scan(&v)
	require.Error(t, err)

	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()
	require.Len(t, mockLog.errorLogs, 1)
	assert.Equal(t, 500, mockLog.errorLogs[0].payload.Status)
	assert.Contains(t, mockLog.errorLogs[0].payload.Error, "connection reset by peer")
}

func TestDB_Exec(t *testing.T) {
	db, mockLog, spanRecorder := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	res, err := db.Exec(ctx, "UPDATE users SET status = $1 WHERE id = $2", "active", 1)
	require.NoError(t, err)
	require.NotNil(t, res)

	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "postgres: Exec", spans[0].Name())

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()
	require.Len(t, mockLog.infoLogs, 1)
	assert.Equal(t, 200, mockLog.infoLogs[0].payload.Status)
}

package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
)

var (
	mockDriverInstance = &mockSQLDriver{}
	driverOnce         sync.Once
)

type mockSQLDriver struct {
	mu          sync.Mutex
	beginErr    error
	commitErr   error
	rollbackErr error
	committed   int
	rolledBack  int
}

func (d *mockSQLDriver) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.beginErr = nil
	d.commitErr = nil
	d.rollbackErr = nil
	d.committed = 0
	d.rolledBack = 0
}

func (d *mockSQLDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{d: d}, nil
}

type mockConn struct {
	d *mockSQLDriver
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *mockConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	if c.d.beginErr != nil {
		return nil, c.d.beginErr
	}
	return &mockTx{d: c.d}, nil
}

type mockTx struct {
	d *mockSQLDriver
}

func (tx *mockTx) Commit() error {
	tx.d.mu.Lock()
	defer tx.d.mu.Unlock()
	if tx.d.commitErr != nil {
		return tx.d.commitErr
	}
	tx.d.committed++
	return nil
}

func (tx *mockTx) Rollback() error {
	tx.d.mu.Lock()
	defer tx.d.mu.Unlock()
	if tx.d.rollbackErr != nil {
		return tx.d.rollbackErr
	}
	tx.d.rolledBack++
	return nil
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	driverOnce.Do(func() {
		sql.Register("mock_tx_driver", mockDriverInstance)
	})
	mockDriverInstance.reset()

	db, err := sql.Open("mock_tx_driver", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	return db
}

func TestRunInTx_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var executed bool
	err := RunInTx(context.Background(), db, func(ctx context.Context, tx *sql.Tx) error {
		if tx == nil {
			t.Fatal("expected non-nil tx")
		}
		if ctxTx := TxFromContext(ctx); ctxTx != tx {
			t.Fatal("expected TxFromContext to match current tx")
		}
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected fn to be executed")
	}

	mockDriverInstance.mu.Lock()
	defer mockDriverInstance.mu.Unlock()
	if mockDriverInstance.committed != 1 {
		t.Errorf("committed = %d, want 1", mockDriverInstance.committed)
	}
	if mockDriverInstance.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0", mockDriverInstance.rolledBack)
	}
}

func TestRunInTx_HandlerError_RollsBack(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	expectedErr := errors.New("business error")
	err := RunInTx(context.Background(), db, func(ctx context.Context, tx *sql.Tx) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("got error %v, want %v", err, expectedErr)
	}

	mockDriverInstance.mu.Lock()
	defer mockDriverInstance.mu.Unlock()
	if mockDriverInstance.committed != 0 {
		t.Errorf("committed = %d, want 0", mockDriverInstance.committed)
	}
	if mockDriverInstance.rolledBack != 1 {
		t.Errorf("rolledBack = %d, want 1", mockDriverInstance.rolledBack)
	}
}

func TestRunInTx_Panic_RollsBackAndRethrows(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	defer func() {
		p := recover()
		if p == nil {
			t.Fatal("expected panic to be rethrown")
		}
		if p != "fatal database logic failure" {
			t.Fatalf("unexpected panic value: %v", p)
		}

		mockDriverInstance.mu.Lock()
		defer mockDriverInstance.mu.Unlock()
		if mockDriverInstance.committed != 0 {
			t.Errorf("committed = %d, want 0", mockDriverInstance.committed)
		}
		if mockDriverInstance.rolledBack != 1 {
			t.Errorf("rolledBack = %d, want 1", mockDriverInstance.rolledBack)
		}
	}()

	_ = RunInTx(context.Background(), db, func(ctx context.Context, tx *sql.Tx) error {
		panic("fatal database logic failure")
	})
}

func TestRunInTx_BeginTxError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mockDriverInstance.mu.Lock()
	mockDriverInstance.beginErr = errors.New("connection failed")
	mockDriverInstance.mu.Unlock()

	err := RunInTx(context.Background(), db, func(ctx context.Context, tx *sql.Tx) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunInTx_CommitError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mockDriverInstance.mu.Lock()
	mockDriverInstance.commitErr = errors.New("commit disk full")
	mockDriverInstance.mu.Unlock()

	err := RunInTx(context.Background(), db, func(ctx context.Context, tx *sql.Tx) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected commit error, got nil")
	}
}

func TestRunInTx_ValidationErrors(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("nil db", func(t *testing.T) {
		err := RunInTx(context.Background(), nil, func(ctx context.Context, tx *sql.Tx) error {
			return nil
		})
		if err == nil || err.Error() != "database: transactor is nil" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil fn", func(t *testing.T) {
		err := RunInTx(context.Background(), db, nil)
		if err == nil || err.Error() != "database: tx function is nil" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := RunInTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	})
}

func TestRunInTx_NestedTransactions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var outerTx, innerTx *sql.Tx
	var innerExecuted bool

	err := RunInTx(context.Background(), db, func(ctx context.Context, tx1 *sql.Tx) error {
		outerTx = tx1

		// Nested RunInTx: should detect existing tx in ctx and reuse it
		return RunInTx(ctx, db, func(nestedCtx context.Context, tx2 *sql.Tx) error {
			innerTx = tx2
			innerExecuted = true
			if tx1 != tx2 {
				return errors.New("nested tx does not match outer tx")
			}
			return nil
		})
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !innerExecuted {
		t.Fatal("expected inner tx to be executed")
	}
	if outerTx == nil || outerTx != innerTx {
		t.Fatalf("expected outer and inner tx to be identical, got outer: %v, inner: %v", outerTx, innerTx)
	}

	mockDriverInstance.mu.Lock()
	defer mockDriverInstance.mu.Unlock()
	// Only 1 commit for the outer transaction!
	if mockDriverInstance.committed != 1 {
		t.Errorf("committed = %d, want 1", mockDriverInstance.committed)
	}
	if mockDriverInstance.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0", mockDriverInstance.rolledBack)
	}
}

func TestTxFromContext_NilContext(t *testing.T) {
	if tx := TxFromContext(nil); tx != nil {
		t.Fatalf("expected nil for nil context, got: %v", tx)
	}

	if tx := TxFromContext(context.Background()); tx != nil {
		t.Fatalf("expected nil for empty context, got: %v", tx)
	}
}

# Database Storage Module (`gosdk/storage/database`)

The `github.com/agilsyofian/gosdk/storage/database` module is an enterprise-grade database abstraction layer engineered for banking, fintech, and high-throughput microservices. Built on Go's standard library `database/sql`, it delivers zero-overhead query execution, deep OpenTelemetry distributed tracing, structured diagnostics, Kubernetes readiness health checks, and robust transaction management (`RunInTx`) with automatic rollback and panic recovery.

---

## 🚀 Key Capabilities & Features

- 🏛️ **Unified `Database` & `Transactor` Contracts**:
  - `database.Database`: Unified interface satisfying `Exec`, `Query`, `QueryRow`, `RunInTx`, `Ping`, `Close`, `Driver`, and `Stats`.
  - `database.Transactor`: Lightweight interface (`BeginTx(ctx, opts)`) satisfied by both `database.Database` and standard `*sql.DB`.
  - Repositories depend purely on interfaces, enabling 100% testability and mocking without requiring real database servers.
- 🛡️ **Transaction Management & Panic Safety (`RunInTx`)**:
  - **Closure-Based Transactions**: Executes operations inside `RunInTx(ctx, db, fn)` or `RunInTxWithOptions(ctx, db, opts, fn)`.
  - **Automatic Commit & Rollback**: Commits on `nil` error, and automatically rolls back if an error is returned or if a panic occurs.
  - **Panic Resilience**: Catches panics, safely rolls back the open transaction to prevent connection leaks in the pool, and re-throws the panic for upstream recovery.
  - **Nested Transaction Support**: When `RunInTx` is called inside an existing transaction context, it automatically participates in the outer transaction without opening redundant connections or committing prematurely.
  - **Context Propagation**: Injects the active transaction into context via `WithTx(ctx, tx)`, retrievable via `TxFromContext(ctx)`.
- 🗄️ **Multi-Engine RDBMS Support**:
  - **PostgreSQL** (`gosdk/storage/postgres` via `github.com/lib/pq`)
  - **MySQL / MariaDB** (`gosdk/storage/mysql` via `github.com/go-sql-driver/mysql`)
  - **Oracle Database** (`gosdk/storage/oracle` via `github.com/sijms/go-ora/v2`, pure Go, no Oracle Client / Instant Client required)
- 🔍 **OpenTelemetry Distributed Tracing (APM)**:
  - Automatic client spans (`trace.SpanKindClient`) on every `Exec`, `Query`, `QueryRow`, and `BeginTx`.
  - Enriched with semantic attributes: `db.system`, `db.operation`, `db.statement`, `db.user`, and `db.name`.
  - Correlates slow queries and database errors directly with the incoming HTTP / gRPC request trace.
- 📊 **Structured Observability & Logging**:
  - Emits structured JSON diagnostics via `core/observability/logger` (`STORAGE_POSTGRES`, `STORAGE_MYSQL`, `STORAGE_ORACLE`).
  - Records `TraceId`, `SpanId`, `SessionId`, `IpAddress`, `Duration` (ms), query statement, and error causes.
- ☸️ **Kubernetes Lifecycle & Health Probes**:
  - Managed lifecycle hooks via **Uber Fx** (`OnStart` performs connection verification and `Ping`, `OnStop` closes pool gracefully).
  - Automatically registers readiness probes into `core/observability/health.Registry` for Kubernetes `/healthz` probes.
- ⚡ **Connection Pool Management**:
  - Granular control over `max_open_conns`, `max_idle_conns`, `conn_max_lifetime`, and `conn_max_idle_time`.

---

## 🏛️ Architecture & Transaction Flow

### Managed Transaction Lifecycle (`RunInTx`)

```mermaid
flowchart TD
    Start([Call RunInTx]) --> CheckCtx{Active Tx in Context?}
    CheckCtx -->|Yes (Nested)| ExecNested[Participate in Existing Tx]
    ExecNested --> ReturnNested[Return to Outer Caller]
    
    CheckCtx -->|No (Root)| Begin[BeginTx on DB Driver]
    Begin --> Inject[Inject Tx into Context: WithTx]
    Inject --> Exec[Execute User Fn]
    
    Exec -->|Panic Caught| Recover[Rollback Tx\nRe-throw Panic]
    Exec -->|Error Returned| Rollback[Rollback Tx\nJoin Errors]
    Exec -->|Success (nil)| Commit[Commit Tx]
    
    Commit --> Done([Success])
    Rollback --> ErrOut([Return Error])
```

---

## ⚙️ Configuration Reference (`env.yaml`)

Configuration is uniform across PostgreSQL, MySQL, and Oracle:

```yaml
storage:
  # PostgreSQL Configuration
  postgres:
    host: "postgres-cluster.db.svc.cluster.local"
    port: 5432
    user: "app_user"
    password: "${DB_PASSWORD}"
    db_name: "order_db"
    ssl_mode: "disable" # disable, require, verify-full
    max_open_conns: 50
    max_idle_conns: 10
    conn_max_lifetime: "30m"
    conn_max_idle_time: "5m"
    tracing:
      enabled: true
    logger:
      enabled: true

  # MySQL Configuration
  mysql:
    host: "mysql-master.db.svc.cluster.local"
    port: 3306
    user: "app_user"
    password: "${DB_PASSWORD}"
    db_name: "inventory_db"
    ssl_mode: "disable"
    max_open_conns: 40
    max_idle_conns: 10
    conn_max_lifetime: "30m"
    conn_max_idle_time: "5m"
    tracing:
      enabled: true
    logger:
      enabled: true

  # Oracle Database Configuration
  oracle:
    host: "oracle-db.internal"
    port: 1521
    user: "CORE_BANKING"
    password: "${ORACLE_PASSWORD}"
    db_name: "ORCLPDB1"        # Service Name or DB Name
    service_name: "ORCLPDB1"   # Optional, alternative to SID
    ssl_mode: "disable"
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: "1h"
    conn_max_idle_time: "10m"
    tracing:
      enabled: true
    logger:
      enabled: true
```

---

## 💻 Code Examples

### 1. Dependency Injection with Uber Fx

Wire your database instance into the dependency injection container:

```go
package bootstrap

import (
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/storage/database"
	"github.com/agilsyofian/gosdk/storage/postgres"
	"go.uber.org/fx"
)

var StorageModule = fx.Module("storage",
	fx.Provide(
		// Provide configuration
		func() postgres.Config {
			return postgres.Config{
				Host:            "localhost",
				Port:            5432,
				User:            "postgres",
				Password:        "secret",
				DBName:          "payment_db",
				SSLMode:         "disable",
				MaxOpenConns:    20,
				MaxIdleConns:    5,
				ConnMaxLifetime: 30 * time.Minute,
			}
		},
		// Factory: pure Go constructor, with lifecycle hooks wired cleanly at bootstrap level
		func(lc fx.Lifecycle, l logger.Logger, cfg postgres.Config, hr health.Registry) (database.Database, error) {
			db, err := postgres.NewPostgresDB(l, cfg, hr)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStart: db.Connect,
				OnStop:  func(ctx context.Context) error { return db.Close() },
			})
			return db, nil
		},
	),
)
```

---

### 2. Standard Querying & Scanning

```go
package repository

import (
	"context"
	"database/sql"
	"github.com/agilsyofian/gosdk/storage/database"
)

type UserRepository struct {
	db database.Database
}

func NewUserRepository(db database.Database) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRow(ctx, "SELECT id, name, email FROM users WHERE id = $1", id)

	var u User
	if err := row.Scan(&u.ID, &u.Name, &u.Email); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
```

---

### 3. Managed Transactions (`RunInTx`)

Perform multiple atomic writes with automatic rollback on error or panic:

```go
package service

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/agilsyofian/gosdk/storage/database"
)

type TransferService struct {
	db database.Database
}

func (s *TransferService) TransferMoney(ctx context.Context, senderID, recipientID string, amount int64) error {
	return database.RunInTx(ctx, s.db, func(txCtx context.Context, tx *sql.Tx) error {
		// 1. Deduct sender balance
		res, err := tx.ExecContext(txCtx, "UPDATE accounts SET balance = balance - $1 WHERE id = $2 AND balance >= $1", amount, senderID)
		if err != nil {
			return fmt.Errorf("deduct balance: %w", err) // Triggers automatic rollback
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("insufficient funds or account not found") // Triggers automatic rollback
		}

		// 2. Add recipient balance
		_, err = tx.ExecContext(txCtx, "UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, recipientID)
		if err != nil {
			return fmt.Errorf("credit balance: %w", err) // Triggers automatic rollback
		}

		// 3. Record transaction audit
		_, err = tx.ExecContext(txCtx, "INSERT INTO transfer_logs (sender, recipient, amount) VALUES ($1, $2, $3)", senderID, recipientID, amount)
		if err != nil {
			return fmt.Errorf("record log: %w", err) // Triggers automatic rollback
		}

		return nil // Automatically commits!
	})
}
```

---

### 4. Context-Aware Repositories with `TxFromContext`

When repositories are called within a service transaction, they can transparently participate in the transaction without changing method signatures:

```go
package repository

import (
	"context"
	"github.com/agilsyofian/gosdk/storage/database"
)

type WalletRepository struct {
	db database.Database
}

func (r *WalletRepository) UpdateBalance(ctx context.Context, accountID string, delta int64) error {
	// If an active transaction exists in ctx (via RunInTx), use it; otherwise use the pool.
	if tx := database.TxFromContext(ctx); tx != nil {
		_, err := tx.ExecContext(ctx, "UPDATE wallets SET balance = balance + $1 WHERE id = $2", delta, accountID)
		return err
	}

	_, err := r.db.Exec(ctx, "UPDATE wallets SET balance = balance + $1 WHERE id = $2", delta, accountID)
	return err
}
```

---

### 5. Nested Transactions

If a method calling `RunInTx` calls another method that also invokes `RunInTx`, the inner call seamlessly participates in the outer transaction:

```go
func (s *OrderService) Checkout(ctx context.Context, order *Order) error {
	return database.RunInTx(ctx, s.db, func(txCtx context.Context, tx *sql.Tx) error {
		// Creates root transaction
		if err := s.orderRepo.CreateOrder(txCtx, order); err != nil {
			return err
		}

		// Inner service method calling RunInTx(txCtx, ...) automatically
		// participates in the existing transaction without opening a second one!
		if err := s.inventoryService.DeductStock(txCtx, order.Items); err != nil {
			return err // Rolls back both CreateOrder and DeductStock!
		}

		return nil // Commits both changes together
	})
}
```

---

## 🛡️ Production Best Practices

1. **Always Set Connection Pool Boundaries**: Never leave `max_open_conns` unbounded. In Kubernetes, multiply `max_open_conns` by the maximum number of HPA replica pods to ensure total connections do not exceed database capacity.
2. **Set `conn_max_lifetime` Below Load Balancer Timeouts**: If using AWS NLB, GCP Cloud SQL Auth Proxy, or Azure Application Gateway, set `conn_max_lifetime` (e.g. `30m`) below proxy idle timeouts to avoid broken pipe errors.
3. **Use `RunInTx` for Multi-Statement Mutations**: Avoid manual `db.BeginTx`, `tx.Commit()`, and `tx.Rollback()` calls. `RunInTx` guarantees panic recovery, prevent uncommitted transaction leaks, and integrates seamlessly with context tracing.
4. **Leverage Health Checks**: Pass `health.Registry` into `postgres.NewPostgresDB(...)` so Kubernetes diverts traffic during database failovers.

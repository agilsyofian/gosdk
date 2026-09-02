# migrator

A lightweight migration runner on top of `database/sql`, built generically
so it can be used directly with (Oracle / PostgreSQL / MySQL)
without relying on a third-party Oracle driver that's often incomplete
(golang-migrate has no official Oracle driver).

## Structure

```
migrator/
  internal/migrate/migrator.go   # core engine (used as a library)
  cmd/migrate/main.go            # CLI
  migrations/                    # *.up.sql / *.down.sql files
```

## 1. Creating a new migration

```bash
go run ./cmd/migrate create create_users_table
```

Produces:

```
migrations/20260712153000_create_users_table.up.sql
migrations/20260712153000_create_users_table.down.sql
```

Versions use a timestamp (instead of a sequential number like 0001, 0002)
so they don't collide when all 6 squads create migrations in parallel on
different branches and then merge.

Fill in the file contents manually, for example:

```sql
-- up
CREATE TABLE accounts (
    id BIGINT PRIMARY KEY,
    balance NUMERIC(18,2) NOT NULL DEFAULT 0
);
```

```sql
-- down
DROP TABLE accounts;
```

### PL/SQL statements (Oracle) containing multiple `;`

If you need a `CREATE OR REPLACE TRIGGER` / `PROCEDURE` whose body itself
contains many semicolons, split it into statements using a custom
delimiter so it doesn't get split incorrectly:

```sql
-- +statement
CREATE OR REPLACE TRIGGER trg_users_audit
BEFORE INSERT ON users
FOR EACH ROW
BEGIN
  :NEW.created_at := SYSTIMESTAMP;
END;
-- +statement
CREATE INDEX idx_users_email ON users (email)
```

## 2. Running migrations

Set the connection via env vars (adapt these to however `godb` reads its
config):

```bash
export DB_DIALECT=postgres        # postgres | mysql | oracle
export DB_DSN="postgres://user:pass@localhost:5432/vello?sslmode=disable"
export MIGRATIONS_DIR=migrations  # optional, default "migrations"
```

Run all pending migrations:

```bash
go run ./cmd/migrate up
```

### Using Make

From `tools/dbmigration`, the same commands are available through the
included Makefile:

```bash
make migrate-status
make migrate-up
make migrate-up-to VERSION=20260829210800
make migrate-down STEPS=1
make migrate-create NAME=create_orders_table
make migrate-force VERSION=20260829210800 STATE=applied
```

Use `make help` to see every target. `DB_DIALECT`, `DB_DSN`, and (optionally)
`MIGRATIONS_DIR` can be provided in `.env` or overridden when calling Make:

```bash
DB_DIALECT=postgres DB_DSN='postgres://user:pass@localhost:5432/app?sslmode=disable' make migrate-status
```

Run up to a specific version only:

```bash
go run ./cmd/migrate up 20260712153000
```

Roll back the last 1 migration:

```bash
go run ./cmd/migrate down
```

Roll back 3 migrations:

```bash
go run ./cmd/migrate down 3
```

Check status:

```bash
go run ./cmd/migrate status
```

```
20260712150000 create_users_table            applied  2026-07-12T15:05:00Z
20260712153000 add_accounts_table            pending
```

If a migration fails midway and the `schema_migrations` table flags it as
`dirty`, the runner will refuse to run further migrations until you fix it
manually and then run:

```bash
go run ./cmd/migrate force 20260712153000 applied
```

## Integrating with godb

`Migrator` only needs a plain `*sql.DB`, so if your `godb` has a method
that exposes the raw connection (`db.Raw()` or similar) per dialect, you
can just pass it to `migrate.New(rawDB, dialect, dir)` without opening a
new connection in `main.go`. For Oracle, keep in mind that DDL auto-commits
— the runner already handles this by flagging `dirty=1` before execution
and `dirty=0` after success, so a mid-run failure is still visible via
`status`.

## Dependencies

```bash
go get github.com/jackc/pgx/v5/stdlib
go get github.com/go-sql-driver/mysql
go get github.com/godror/godror   # requires Oracle Instant Client installed
```

If a particular squad doesn't need Oracle, just remove the `godror` import
from `cmd/migrate/main.go` so they don't need to install the Instant
Client on their machine.

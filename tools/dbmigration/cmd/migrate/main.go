// Command migrate is a small CLI around internal/migrate.
//
// Usage:
//
//	go run ./cmd/migrate create create_users_table
//	go run ./cmd/migrate up
//	go run ./cmd/migrate up 20260712153000      // up to a specific version
//	go run ./cmd/migrate down                   // rollback 1
//	go run ./cmd/migrate down 3                 // rollback 3
//	go run ./cmd/migrate status
//	go run ./cmd/migrate force 20260712153000 applied
//	go run ./cmd/migrate force 20260712153000 unapplied
//
// Connection is configured via env vars so it stays consistent with how
// godb picks up connection info per-dialect:
//
//	DB_DIALECT   = postgres | mysql | oracle
//	DB_DSN       = full driver DSN for the chosen dialect
//	MIGRATIONS_DIR = path to migrations folder (default "migrations")
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/agilsyofian/gosdk/tools/dbmigration/internal/migrate"
	"github.com/joho/godotenv"

	_ "github.com/go-sql-driver/mysql" // mysql driver
	_ "github.com/godror/godror"       // oracle driver
	_ "github.com/jackc/pgx/v5/stdlib" // postgres driver
)

func main() {

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	dir := envOr("MIGRATIONS_DIR", "migrations")

	switch os.Args[1] {
	case "create":
		if len(os.Args) < 3 {
			log.Fatal("usage: migrate create <name>")
		}
		name := os.Args[2]
		up, down, err := migrate.CreateMigration(dir, name)
		if err != nil {
			log.Fatalf("create migration: %v", err)
		}
		fmt.Println("created:")
		fmt.Println("  " + up)
		fmt.Println("  " + down)
		return

	case "up", "down", "status", "force":
		m := mustMigrator(dir)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		runDBCommand(ctx, m, os.Args[1], os.Args[2:])

	default:
		usage()
		os.Exit(1)
	}
}

func runDBCommand(ctx context.Context, m *migrate.Migrator, cmd string, args []string) {
	switch cmd {
	case "up":
		var target int64
		if len(args) > 0 {
			v, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				log.Fatalf("invalid version %q: %v", args[0], err)
			}
			target = v
		}
		n, err := m.UpTo(ctx, target)
		if err != nil {
			log.Fatalf("up: %v", err)
		}
		fmt.Printf("applied %d migration(s)\n", n)

	case "down":
		steps := 1
		if len(args) > 0 {
			v, err := strconv.Atoi(args[0])
			if err != nil {
				log.Fatalf("invalid steps %q: %v", args[0], err)
			}
			steps = v
		}
		n, err := m.Down(ctx, steps)
		if err != nil {
			log.Fatalf("down: %v", err)
		}
		fmt.Printf("rolled back %d migration(s)\n", n)

	case "status":
		rows, err := m.Status(ctx)
		if err != nil {
			log.Fatalf("status: %v", err)
		}
		if len(rows) == 0 {
			fmt.Println("no migrations found")
			return
		}
		for _, r := range rows {
			state := "pending"
			if r.Applied {
				state = "applied"
				if r.Dirty {
					state = "DIRTY"
				}
			}
			ts := ""
			if r.AppliedAt != nil {
				ts = r.AppliedAt.Format(time.RFC3339)
			}
			fmt.Printf("%-14d %-30s %-8s %s\n", r.Version, r.Name, state, ts)
		}

	case "force":
		if len(args) < 2 {
			log.Fatal("usage: migrate force <version> <applied|unapplied>")
		}
		v, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			log.Fatalf("invalid version %q: %v", args[0], err)
		}
		applied := args[1] == "applied"
		if err := m.Force(ctx, v, applied); err != nil {
			log.Fatalf("force: %v", err)
		}
		fmt.Printf("forced version %d to %s\n", v, args[1])
	}
}

func mustMigrator(dir string) *migrate.Migrator {
	dialectStr := envOr("DB_DIALECT", "")
	dsn := envOr("DB_DSN", "")
	if dialectStr == "" || dsn == "" {
		log.Fatal("DB_DIALECT and DB_DSN env vars are required (DB_DIALECT: postgres|mysql|oracle)")
	}

	var driverName string
	var dialect migrate.Dialect
	switch dialectStr {
	case "postgres":
		driverName, dialect = "pgx", migrate.Postgres
	case "mysql":
		driverName, dialect = "mysql", migrate.MySQL
	case "oracle":
		driverName, dialect = "godror", migrate.Oracle
	default:
		log.Fatalf("unsupported DB_DIALECT %q", dialectStr)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	return migrate.New(db, dialect, dir)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Println(`migrate — simple multi-dialect migration runner

Usage:
  migrate create <name>
  migrate up [target_version]
  migrate down [steps]
  migrate status
  migrate force <version> <applied|unapplied>

Env:
  DB_DIALECT      postgres | mysql | oracle
  DB_DSN          driver DSN
  MIGRATIONS_DIR  default "migrations"`)
}

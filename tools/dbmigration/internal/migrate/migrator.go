// Package migrate implements a lightweight, dialect-aware SQL migration
// runner on top of database/sql. It is intentionally dependency-free so it
// can be dropped into any project (e.g. godb) that already manages its own
// *sql.DB / *sql.Conn for Oracle, PostgreSQL, or MySQL.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Dialect identifies the target database so the runner can generate the
// correct DDL for the schema_migrations bookkeeping table and quoting.
type Dialect string

const (
	Postgres Dialect = "postgres"
	MySQL    Dialect = "mysql"
	Oracle   Dialect = "oracle"
)

// Migration represents a single versioned migration on disk.
type Migration struct {
	Version  int64  // timestamp-based version, e.g. 20260712153000
	Name     string // human readable slug
	UpFile   string // absolute path to *.up.sql
	DownFile string // absolute path to *.down.sql
}

// AppliedMigration is a row from the schema_migrations table.
type AppliedMigration struct {
	Version   int64
	Dirty     bool
	AppliedAt time.Time
}

// Migrator ties a *sql.DB, a dialect, and a migrations directory together.
type Migrator struct {
	DB      *sql.DB
	Dialect Dialect
	Dir     string
}

// New creates a Migrator. dir is the folder containing *.up.sql / *.down.sql files.
func New(db *sql.DB, dialect Dialect, dir string) *Migrator {
	return &Migrator{DB: db, Dialect: dialect, Dir: dir}
}

var fileNameRe = regexp.MustCompile(`^(\d{14,})_([a-zA-Z0-9_\-]+)\.(up|down)\.sql$`)

// ---------------------------------------------------------------------
// 1. Creating new migrations
// ---------------------------------------------------------------------

// CreateMigration generates a new pair of empty migration files:
//
//	<dir>/<timestamp>_<name>.up.sql
//	<dir>/<timestamp>_<name>.down.sql
//
// The timestamp guarantees migrations sort and apply in creation order,
// even across parallel feature branches (avoids merge collisions that
// sequential integer prefixes like 0001, 0002 tend to cause in multi-squad
// repos).
func CreateMigration(dir, name string) (upPath, downPath string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create migrations dir: %w", err)
	}

	slug := slugify(name)
	if slug == "" {
		return "", "", fmt.Errorf("migration name cannot be empty after slugify")
	}

	version := time.Now().UTC().Format("20060102150405")
	base := fmt.Sprintf("%s_%s", version, slug)

	upPath = filepath.Join(dir, base+".up.sql")
	downPath = filepath.Join(dir, base+".down.sql")

	upStub := fmt.Sprintf("-- +migrate Up\n-- migration: %s\n-- created_at: %s\n\n", base, time.Now().UTC().Format(time.RFC3339))
	downStub := fmt.Sprintf("-- +migrate Down\n-- migration: %s\n-- created_at: %s\n\n", base, time.Now().UTC().Format(time.RFC3339))

	if err := os.WriteFile(upPath, []byte(upStub), 0o644); err != nil {
		return "", "", fmt.Errorf("write up file: %w", err)
	}
	if err := os.WriteFile(downPath, []byte(downStub), 0o644); err != nil {
		return "", "", fmt.Errorf("write down file: %w", err)
	}
	return upPath, downPath, nil
}

func slugify(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "_")
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------
// 2. Discovering migrations on disk
// ---------------------------------------------------------------------

func (m *Migrator) loadMigrations() ([]Migration, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	byVersion := map[int64]*Migration{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		match := fileNameRe.FindStringSubmatch(e.Name())
		if match == nil {
			fmt.Printf("WARNING: ignoring non-migration file %s\n", e.Name())
			continue // skip files that don't look like <version>_<name>.(up|down).sql
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad version in %s: %w", e.Name(), err)
		}
		mig, ok := byVersion[version]
		if !ok {
			mig = &Migration{Version: version, Name: match[2]}
			byVersion[version] = mig
		}
		full := filepath.Join(m.Dir, e.Name())
		if match[3] == "up" {
			mig.UpFile = full
		} else {
			mig.DownFile = full
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, mig := range byVersion {
		if mig.UpFile == "" || mig.DownFile == "" {
			return nil, fmt.Errorf("migration %d (%s) is missing its up or down file", mig.Version, mig.Name)
		}
		out = append(out, *mig)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// ---------------------------------------------------------------------
// 3. Bookkeeping table (dialect-specific DDL)
// ---------------------------------------------------------------------

func (m *Migrator) ensureSchemaTable(ctx context.Context) error {
	var ddl string
	switch m.Dialect {
	case Postgres:
		ddl = `CREATE TABLE IF NOT EXISTS schema_migrations (
			version     BIGINT PRIMARY KEY,
			dirty       BOOLEAN NOT NULL DEFAULT FALSE,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`
	case MySQL:
		ddl = `CREATE TABLE IF NOT EXISTS schema_migrations (
			version     BIGINT PRIMARY KEY,
			dirty       TINYINT(1) NOT NULL DEFAULT 0,
			applied_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`
	case Oracle:
		// Oracle: no IF NOT EXISTS on CREATE TABLE, no BOOLEAN type,
		// and DDL auto-commits (no transactional rollback for schema
		// changes), so this must be idempotent by checking first.
		ddl = `BEGIN
			EXECUTE IMMEDIATE 'CREATE TABLE schema_migrations (
				version     NUMBER(19) PRIMARY KEY,
				dirty       NUMBER(1) DEFAULT 0 NOT NULL,
				applied_at  TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL
			)';
		EXCEPTION
			WHEN OTHERS THEN
				IF SQLCODE != -955 THEN -- ORA-00955: name already used by an existing object
					RAISE;
				END IF;
		END;`
	default:
		return fmt.Errorf("unsupported dialect: %s", m.Dialect)
	}

	_, err := m.DB.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func (m *Migrator) appliedMigrations(ctx context.Context) (map[int64]AppliedMigration, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT version, dirty, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	out := map[int64]AppliedMigration{}
	for rows.Next() {
		var am AppliedMigration
		var dirtyRaw interface{}
		if err := rows.Scan(&am.Version, &dirtyRaw, &am.AppliedAt); err != nil {
			return nil, err
		}
		am.Dirty = asBool(dirtyRaw)
		out[am.Version] = am
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------
// 4. Running migrations
// ---------------------------------------------------------------------

// Up applies every pending migration in order. It stops (returning an
// error) at the first migration that leaves the DB in a "dirty" state.
func (m *Migrator) Up(ctx context.Context) (applied int, err error) {
	return m.UpTo(ctx, 0) // 0 == no ceiling, apply everything pending
}

// UpTo applies pending migrations up to and including targetVersion.
// Pass 0 to apply all pending migrations.
func (m *Migrator) UpTo(ctx context.Context, targetVersion int64) (applied int, err error) {
	if err := m.ensureSchemaTable(ctx); err != nil {
		return 0, err
	}
	if err := m.failIfDirty(ctx); err != nil {
		return 0, err
	}

	all, err := m.loadMigrations()
	if err != nil {
		return 0, err
	}
	done, err := m.appliedMigrations(ctx)
	if err != nil {
		return 0, err
	}

	for _, mig := range all {
		if _, ok := done[mig.Version]; ok {
			continue // already applied
		}
		if targetVersion != 0 && mig.Version > targetVersion {
			break
		}
		if err := m.runOne(ctx, mig.Version, mig.UpFile, true); err != nil {
			return applied, fmt.Errorf("apply %d_%s: %w", mig.Version, mig.Name, err)
		}
		applied++
	}
	return applied, nil
}

// Down rolls back `steps` most-recently-applied migrations (default 1).
func (m *Migrator) Down(ctx context.Context, steps int) (rolledBack int, err error) {
	if steps <= 0 {
		steps = 1
	}
	if err := m.ensureSchemaTable(ctx); err != nil {
		return 0, err
	}
	if err := m.failIfDirty(ctx); err != nil {
		return 0, err
	}

	all, err := m.loadMigrations()
	if err != nil {
		return 0, err
	}
	migByVersion := map[int64]Migration{}
	for _, mg := range all {
		migByVersion[mg.Version] = mg
	}

	done, err := m.appliedMigrations(ctx)
	if err != nil {
		return 0, err
	}
	appliedVersions := make([]int64, 0, len(done))
	for v := range done {
		appliedVersions = append(appliedVersions, v)
	}
	sort.Slice(appliedVersions, func(i, j int) bool { return appliedVersions[i] > appliedVersions[j] })

	for i := 0; i < steps && i < len(appliedVersions); i++ {
		v := appliedVersions[i]
		mig, ok := migByVersion[v]
		if !ok {
			return rolledBack, fmt.Errorf("no migration file found for applied version %d (schema_migrations table out of sync with %s)", v, m.Dir)
		}
		if err := m.runOne(ctx, v, mig.DownFile, false); err != nil {
			return rolledBack, fmt.Errorf("rollback %d_%s: %w", mig.Version, mig.Name, err)
		}
		rolledBack++
	}
	return rolledBack, nil
}

// runOne executes a single migration file and updates schema_migrations.
//
// NOTE on Oracle: DDL statements auto-commit, so a mid-script failure can
// leave partial changes applied even though we mark the row dirty inside
// what looks like "one transaction". For Postgres/MySQL (with a
// transactional DDL-capable engine like InnoDB), the up/down script itself
// runs inside a single DB transaction, so a failure rolls back cleanly.
func (m *Migrator) runOne(ctx context.Context, version int64, file string, up bool) error {
	sqlBytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}
	stmts := splitStatements(string(sqlBytes))
	if len(stmts) == 0 {
		return fmt.Errorf("migration file %s has no SQL statements", file)
	}

	if m.Dialect == Oracle {
		// Oracle: run outside an explicit sql.Tx since DDL auto-commits
		// anyway; mark dirty first so a crash mid-migration is visible.
		if up {
			if _, err := m.DB.ExecContext(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES (:1, 1)`, version); err != nil {
				return fmt.Errorf("mark dirty: %w", err)
			}
		}
		for _, stmt := range stmts {
			if _, err := m.DB.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("exec statement failed, DB left DIRTY at version %d — fix manually then use Force(): %w", version, err)
			}
		}
		if up {
			if _, err := m.DB.ExecContext(ctx, `UPDATE schema_migrations SET dirty = 0 WHERE version = :1`, version); err != nil {
				return fmt.Errorf("clear dirty flag: %w", err)
			}
		} else {
			if _, err := m.DB.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = :1`, version); err != nil {
				return fmt.Errorf("remove schema_migrations row: %w", err)
			}
		}
		return nil
	}

	// Postgres / MySQL: run the whole migration + bookkeeping update in
	// one transaction so a failure never leaves a half-applied migration.
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op if committed

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec statement failed (rolled back): %w", err)
		}
	}

	if up {
		if _, err := tx.ExecContext(ctx, m.placeholder(`INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)`), version, false); err != nil {
			return fmt.Errorf("record migration: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, m.placeholder(`DELETE FROM schema_migrations WHERE version = ?`), version); err != nil {
			return fmt.Errorf("remove schema_migrations row: %w", err)
		}
	}

	return tx.Commit()
}

// placeholder rewrites `?` into `$1, $2, ...` for Postgres; MySQL keeps `?` as-is.
func (m *Migrator) placeholder(query string) string {
	if m.Dialect != Postgres {
		return query
	}
	n := 0
	var b strings.Builder
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// failIfDirty refuses to run further migrations if a previous run crashed
// mid-way, forcing an explicit human decision (Force) instead of silently
// compounding the problem.
func (m *Migrator) failIfDirty(ctx context.Context) error {
	rows, err := m.DB.QueryContext(ctx, `SELECT version, dirty FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("check dirty state: %w", err)
	}
	defer rows.Close()
	var dirtyVersions []int64
	for rows.Next() {
		var v int64
		var dirtyRaw interface{}
		if err := rows.Scan(&v, &dirtyRaw); err != nil {
			return err
		}
		if asBool(dirtyRaw) {
			dirtyVersions = append(dirtyVersions, v)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("check dirty state: %w", err)
	}
	if len(dirtyVersions) > 0 {
		return fmt.Errorf("database is dirty at version(s) %v: fix the schema manually then call Force(version) before running migrations again", dirtyVersions)
	}
	return nil
}

// asBool normalizes the "dirty" column across dialects: Postgres returns a
// real bool, MySQL/Oracle typically return an integer (or []byte from some
// drivers when scanned generically).
func asBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int32:
		return t != 0
	case int:
		return t != 0
	case []byte:
		s := string(t)
		return s == "1" || strings.EqualFold(s, "true")
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case nil:
		return false
	default:
		return false
	}
}

// Force marks a version as applied/clean without executing its SQL. Use
// this after manually fixing a dirty migration.
func (m *Migrator) Force(ctx context.Context, version int64, applied bool) error {
	if err := m.ensureSchemaTable(ctx); err != nil {
		return err
	}
	if applied {
		_, err := m.DB.ExecContext(ctx, m.placeholder(`DELETE FROM schema_migrations WHERE version = ?`), version)
		if err != nil {
			return err
		}
		_, err = m.DB.ExecContext(ctx, m.placeholder(`INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)`), version, false)
		return err
	}
	_, err := m.DB.ExecContext(ctx, m.placeholder(`DELETE FROM schema_migrations WHERE version = ?`), version)
	return err
}

// ---------------------------------------------------------------------
// 5. Status reporting
// ---------------------------------------------------------------------

// StatusRow describes one migration's state for CLI/status output.
type StatusRow struct {
	Version   int64
	Name      string
	Applied   bool
	Dirty     bool
	AppliedAt *time.Time
}

func (m *Migrator) Status(ctx context.Context) ([]StatusRow, error) {
	if err := m.ensureSchemaTable(ctx); err != nil {
		return nil, err
	}
	all, err := m.loadMigrations()
	if err != nil {
		return nil, err
	}
	done, err := m.appliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]StatusRow, 0, len(all))
	for _, mig := range all {
		sr := StatusRow{Version: mig.Version, Name: mig.Name}
		if am, ok := done[mig.Version]; ok {
			sr.Applied = true
			sr.Dirty = am.Dirty
			t := am.AppliedAt
			sr.AppliedAt = &t
		}
		rows = append(rows, sr)
	}
	return rows, nil
}

// splitStatements splits a migration file into individual statements on
// `;` terminators, skipping empty/comment-only fragments. Good enough for
// straightforward DDL/DML; for PL/SQL blocks (Oracle triggers, procedures)
// separate them with a line containing only "-- +statement" as a custom
// delimiter marker instead of relying on ';' (see README).
func splitStatements(script string) []string {
	const delim = "-- +statement"
	var chunks []string
	if strings.Contains(script, delim) {
		chunks = strings.Split(script, delim)
	} else {
		chunks = splitSQLStatements(script)
	}

	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		trimmed := strings.TrimSpace(stripComments(c))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

// splitSQLStatements splits on terminators outside SQL strings and comments.
// This keeps ordinary SQL comments and values such as 'a;b' intact.
func splitSQLStatements(script string) []string {
	var chunks []string
	var statement strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(script); i++ {
		ch := script[i]
		next := byte(0)
		if i+1 < len(script) {
			next = script[i+1]
		}

		if inLineComment {
			statement.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			statement.WriteByte(ch)
			if ch == '*' && next == '/' {
				statement.WriteByte(next)
				i++
				inBlockComment = false
			}
			continue
		}
		if inSingleQuote {
			statement.WriteByte(ch)
			if ch == '\'' {
				if next == '\'' { // escaped single quote
					statement.WriteByte(next)
					i++
				} else {
					inSingleQuote = false
				}
			}
			continue
		}
		if inDoubleQuote {
			statement.WriteByte(ch)
			if ch == '"' {
				if next == '"' { // escaped double quote
					statement.WriteByte(next)
					i++
				} else {
					inDoubleQuote = false
				}
			}
			continue
		}

		switch {
		case ch == '-' && next == '-':
			statement.WriteByte(ch)
			statement.WriteByte(next)
			i++
			inLineComment = true
		case ch == '/' && next == '*':
			statement.WriteByte(ch)
			statement.WriteByte(next)
			i++
			inBlockComment = true
		case ch == '\'':
			statement.WriteByte(ch)
			inSingleQuote = true
		case ch == '"':
			statement.WriteByte(ch)
			inDoubleQuote = true
		case ch == ';':
			chunks = append(chunks, statement.String())
			statement.Reset()
		default:
			statement.WriteByte(ch)
		}
	}

	chunks = append(chunks, statement.String())
	return chunks
}

func stripComments(block string) string {
	lines := strings.Split(block, "\n")
	var kept []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "--") {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}

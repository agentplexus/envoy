// Package store opens the team database and scopes every query to the
// authenticated user. It supports two dialects:
//
//   - PostgreSQL (team mode): row-level security is the isolation backstop.
//     Two roles are involved — the MIGRATION role (table owner) applies schema
//     and policies (owners bypass RLS, which the SECURITY DEFINER membership
//     helpers rely on), and the APPLICATION role (non-owner, default
//     "omniagent_app") the running server connects as; ENABLE ROW LEVEL
//     SECURITY binds every policy for it. Application queries run inside AsUser
//     or AsSystem, which set the transaction-local GUCs the policies read.
//   - SQLite (personal mode): a single implicit user, so there is nothing to
//     isolate. The RLS function/policy/grant migrations are skipped and
//     AsUser/AsSystem are pass-throughs — the same service layer runs against
//     both dialects unchanged.
//
// The raw database handle is unexported so no query path can bypass scoping.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	_ "modernc.org/sqlite"             // registers the pure-Go "sqlite" driver

	"github.com/plexusone/omniagent/team/ent"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrateLockKey is the advisory lock guarding concurrent migrators.
const migrateLockKey = 9021741003

// DefaultAppRole is the non-owner role the application connects as.
const DefaultAppRole = "omniagent_app"

// appRolePattern constrains the configurable role name to a safe identifier.
var appRolePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Config configures the team store.
type Config struct {
	// AppDSN is the application connection string (non-owner role).
	AppDSN string

	// MigrateDSN is the owner connection string used only by Migrate.
	// Empty falls back to AppDSN with a warning — acceptable for dev,
	// wrong for production (the app role must not own tables).
	MigrateDSN string

	// AppRole is the application role name granted table access by the
	// migrations. Defaults to DefaultAppRole.
	AppRole string

	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

func (c *Config) setDefaults() error {
	if c.AppRole == "" {
		c.AppRole = DefaultAppRole
	}
	if !appRolePattern.MatchString(c.AppRole) {
		return fmt.Errorf("invalid app role name %q", c.AppRole)
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// Store is the dialect-aware, scoped team database handle.
type Store struct {
	client  *ent.Client
	db      *sql.DB // unexported: all access goes through AsUser/AsSystem
	dialect string  // dialect.Postgres or dialect.SQLite
	logger  *slog.Logger
}

// dialectFromDSN infers the Ent dialect from a connection string. Postgres
// URLs and key/value DSNs are recognized explicitly; everything else (a file
// path, "file:", "sqlite:", ":memory:") is treated as SQLite — the personal
// mode default.
func dialectFromDSN(dsn string) string {
	l := strings.ToLower(strings.TrimSpace(dsn))
	switch {
	case strings.HasPrefix(l, "postgres://"), strings.HasPrefix(l, "postgresql://"):
		return dialect.Postgres
	case strings.Contains(l, "host=") && (strings.Contains(l, "dbname=") || strings.Contains(l, "user=")):
		return dialect.Postgres
	default:
		return dialect.SQLite
	}
}

// sqliteDSN normalizes a SQLite connection string for the modernc driver: it
// strips an optional sqlite scheme, defaults an empty value to a local file,
// and enables foreign-key enforcement (off by default in SQLite).
func sqliteDSN(dsn string) string {
	dsn = strings.TrimPrefix(dsn, "sqlite://")
	dsn = strings.TrimPrefix(dsn, "sqlite:")
	if dsn == "" {
		dsn = "omniagent.db"
	}
	if !strings.Contains(dsn, "_pragma=foreign_keys") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "_pragma=foreign_keys(1)"
	}
	return dsn
}

// Open connects to the team database as the application role.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	if err := cfg.setDefaults(); err != nil {
		return nil, err
	}
	if cfg.AppDSN == "" {
		return nil, fmt.Errorf("team store: AppDSN is required")
	}

	dia := dialectFromDSN(cfg.AppDSN)

	var (
		db  *sql.DB
		err error
	)
	if dia == dialect.SQLite {
		db, err = sql.Open("sqlite", sqliteDSN(cfg.AppDSN))
	} else {
		db, err = sql.Open("pgx", cfg.AppDSN)
	}
	if err != nil {
		return nil, fmt.Errorf("open team database: %w", err)
	}
	// A file-backed SQLite database serializes writes; a single connection
	// avoids "database is locked" under the personal server's concurrency.
	if dia == dialect.SQLite {
		db.SetMaxOpenConns(1)
	}
	if err := db.PingContext(ctx); err != nil {
		if cerr := db.Close(); cerr != nil {
			cfg.Logger.Error("close team database after failed ping", "error", cerr)
		}
		return nil, fmt.Errorf("ping team database: %w", err)
	}

	drv := entsql.OpenDB(dia, db)
	return &Store{
		client:  ent.NewClient(ent.Driver(drv)),
		db:      db,
		dialect: dia,
		logger:  cfg.Logger,
	}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.client.Close()
}

// AsUser runs fn inside a transaction scoped to the given user. Every query
// fn performs is subject to that user's row-level security view.
func (s *Store) AsUser(ctx context.Context, userID uuid.UUID, superadmin bool, fn func(ctx context.Context, tx *ent.Tx) error) error {
	return s.scoped(ctx, userID.String(), superadmin, false, fn)
}

// AsSystem runs fn inside a transaction with the system (auth layer / agent)
// context: full access to auth tables and agent message inserts, per policy.
// Use only from the auth layer and agent plumbing — never for user requests.
func (s *Store) AsSystem(ctx context.Context, fn func(ctx context.Context, tx *ent.Tx) error) error {
	return s.scoped(ctx, "", false, true, fn)
}

// scoped opens a transaction, sets the RLS GUCs transaction-locally, runs
// fn, and commits (or rolls back on error/panic).
func (s *Store) scoped(ctx context.Context, userID string, superadmin, system bool, fn func(ctx context.Context, tx *ent.Tx) error) (err error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin team tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rerr := tx.Rollback(); rerr != nil {
				s.logger.Error("rollback after panic failed", "error", rerr)
			}
			panic(p)
		}
		if err != nil {
			if rerr := tx.Rollback(); rerr != nil {
				s.logger.Error("rollback failed", "error", rerr)
			}
		}
	}()

	// Postgres: set_config(..., true) scopes the GUC to this transaction only,
	// which the RLS policies read. SQLite (personal mode) has one implicit
	// user and no policies, so scoping is a no-op.
	if s.dialect == dialect.Postgres {
		if _, err = tx.ExecContext(ctx,
			`SELECT set_config('app.current_user_id', $1, true),
			        set_config('app.is_superadmin',  $2, true),
			        set_config('app.is_system',      $3, true)`,
			userID, fmt.Sprint(superadmin), fmt.Sprint(system),
		); err != nil {
			return fmt.Errorf("set rls context: %w", err)
		}
	}

	if err = fn(ctx, tx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit team tx: %w", err)
	}
	return nil
}

// Migrate applies the schema and RLS policies using the owner connection:
// citext extension, Ent-generated tables, then the embedded SQL files
// (functions, policies, grants) in name order. Everything is idempotent and
// serialized by an advisory lock, so concurrent restarts are safe.
func Migrate(ctx context.Context, cfg Config) error {
	if err := cfg.setDefaults(); err != nil {
		return err
	}

	// SQLite (personal mode): a single implicit user means no RLS. The owner/
	// app role split, citext extension, advisory lock, and policy/grant SQL
	// files are all Postgres concepts — skip them and create the schema only.
	if dialectFromDSN(cfg.AppDSN) == dialect.SQLite {
		return sqliteMigrate(ctx, sqliteDSN(cfg.AppDSN))
	}

	dsn := cfg.MigrateDSN
	if dsn == "" {
		cfg.Logger.Warn("team store: MigrateDSN not set; migrating over AppDSN — the app role should not own tables in production")
		dsn = cfg.AppDSN
	}
	if dsn == "" {
		return fmt.Errorf("team store: no DSN to migrate with")
	}

	// Simple-protocol connection so multi-statement migration files execute
	// as single batches (dollar-quoted bodies included).
	pcfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse migrate dsn: %w", err)
	}
	pcfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	conn, err := pgx.ConnectConfig(ctx, pcfg)
	if err != nil {
		return fmt.Errorf("connect for migrate: %w", err)
	}
	defer func() {
		if cerr := conn.Close(ctx); cerr != nil {
			cfg.Logger.Error("close migrate connection", "error", cerr)
		}
	}()

	if _, err := conn.Exec(ctx, fmt.Sprintf("SELECT pg_advisory_lock(%d)", migrateLockKey)); err != nil {
		return fmt.Errorf("acquire migrate lock: %w", err)
	}
	defer func() {
		if _, uerr := conn.Exec(ctx, fmt.Sprintf("SELECT pg_advisory_unlock(%d)", migrateLockKey)); uerr != nil {
			cfg.Logger.Error("release migrate lock", "error", uerr)
		}
	}()

	// 1. Extensions must precede Ent's citext columns.
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS citext"); err != nil {
		return fmt.Errorf("create citext extension: %w", err)
	}

	// 2. Ent auto-migration for tables/indexes/constraints.
	if err := entMigrate(ctx, dsn); err != nil {
		return fmt.Errorf("ent migrate: %w", err)
	}

	// 3. Embedded SQL: functions, policies, grants (idempotent, in order).
	names, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := migrationFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		sqlText := strings.ReplaceAll(string(raw), "{{APP_ROLE}}", cfg.AppRole)
		if _, err := conn.Exec(ctx, sqlText); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		cfg.Logger.Info("applied team migration", "file", name)
	}

	return nil
}

// entMigrate runs Ent's schema creation over a standard database/sql
// connection on the owner DSN.
func entMigrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }() //nolint:errcheck // best-effort close of a migration-only handle

	drv := entsql.OpenDB(dialect.Postgres, db)
	cli := ent.NewClient(ent.Driver(drv))
	return cli.Schema.Create(ctx)
}

// sqliteMigrate creates the Ent schema on a SQLite database. Personal mode has
// no RLS, so this is the whole migration: the citext SchemaType falls back to
// TEXT and the partial "one DM per user per agent" index is emitted as a
// SQLite partial index.
func sqliteMigrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }() //nolint:errcheck // best-effort close of a migration-only handle

	drv := entsql.OpenDB(dialect.SQLite, db)
	cli := ent.NewClient(ent.Driver(drv))
	return cli.Schema.Create(ctx)
}

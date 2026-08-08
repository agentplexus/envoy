// Package pgtest provisions an isolated PostgreSQL database per test suite,
// so the team packages' suites can run in parallel against one dev server
// (see deploy/team/dev/docker-compose.dev.yaml) without clobbering each
// other's schema.
package pgtest

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx database/sql driver
)

// DSNs returns owner and app DSNs pointing at a freshly created database
// that is dropped at test cleanup. The suite is skipped when the dev
// PostgreSQL env vars are unset.
func DSNs(t *testing.T) (ownerDSN, appDSN string) {
	t.Helper()
	baseOwner := os.Getenv("TEAM_TEST_OWNER_DSN")
	baseApp := os.Getenv("TEAM_TEST_APP_DSN")
	if baseOwner == "" || baseApp == "" {
		t.Skip("set TEAM_TEST_OWNER_DSN and TEAM_TEST_APP_DSN to run PostgreSQL-backed team tests (see deploy/team/dev/docker-compose.dev.yaml)")
	}

	// Database identifiers cannot be parameterized; generate a safe name.
	dbName := "team_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	admin, err := sql.Open("pgx", baseOwner)
	if err != nil {
		t.Fatalf("pgtest: open owner: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + dbName); err != nil {
		_ = admin.Close() //nolint:errcheck // fatal path
		t.Fatalf("pgtest: create database: %v", err)
	}
	t.Cleanup(func() {
		// Terminate stragglers, then drop.
		_, _ = admin.Exec(fmt.Sprintf( //nolint:errcheck // best-effort teardown
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()", dbName))
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
			t.Errorf("pgtest: drop database %s: %v", dbName, err)
		}
		if err := admin.Close(); err != nil {
			t.Errorf("pgtest: close admin: %v", err)
		}
	})

	return withDatabase(t, baseOwner, dbName), withDatabase(t, baseApp, dbName)
}

// withDatabase rewrites the database path of a DSN.
func withDatabase(t *testing.T, dsn, dbName string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("pgtest: parse dsn: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
}

// Package pgtest provides a PostgreSQL-backed store for tests.
//
// If TEST_POSTGRES_DSN is set, it connects directly and creates an isolated
// schema per test. Otherwise it spins up a throwaway PostgreSQL container
// via testcontainers.
package pgtest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage/postgres"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationsDir returns the absolute path to the repo-level migrations/
// directory by navigating from this source file's location.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations")
}

// NewStore returns a *postgres.Store backed by PostgreSQL.
//
// When TEST_POSTGRES_DSN is set the store creates an isolated schema per
// test to avoid interference between concurrent tests. Otherwise a
// disposable PostgreSQL 16 container is started via testcontainers.
// In both cases migrations are applied automatically and t.Cleanup
// drops the schema (or terminates the container).
func NewStore(t *testing.T) *postgres.Store {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = startContainer(t)
	}

	migrations := os.Getenv("TEST_POSTGRES_MIGRATIONS")
	if migrations == "" {
		migrations = migrationsDir()
	}

	dsn = isolatedSchema(t, dsn)

	store, err := postgres.New(dsn, migrations)
	if err != nil {
		t.Fatalf("pgtest: create store: %v", err)
	}

	t.Cleanup(func() {
		_ = store.Close()
	})

	return store
}

// isolatedSchema creates a unique PostgreSQL schema for this test and
// returns a modified DSN that uses it via search_path. The schema is
// dropped when the test finishes.
func isolatedSchema(t *testing.T, dsn string) string {
	t.Helper()

	// Generate a unique schema name from the test name.
	schema := "test_" + sanitizeName(t.Name()) + fmt.Sprintf("_%d", time.Now().UnixNano())

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("pgtest: open for schema creation: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("pgtest: create schema %s: %v", schema, err)
	}

	t.Cleanup(func() {
		db2, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Logf("pgtest: open for schema drop: %v", err)
			return
		}
		defer func() { _ = db2.Close() }()
		if _, err := db2.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Logf("pgtest: drop schema %s: %v", schema, err)
		}
	})

	// Add search_path to DSN.
	if strings.Contains(dsn, "?") {
		return dsn + "&search_path=" + schema
	}
	return dsn + "?search_path=" + schema
}

// sanitizeName converts a test name into a valid PostgreSQL identifier.
func sanitizeName(name string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 30 {
		s = s[:30]
	}
	return s
}

// startContainer launches a throwaway PostgreSQL 16 container and returns its
// DSN. The container is terminated when the test finishes.
func startContainer(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	const (
		dbName = "carwatch_test"
		dbUser = "test"
		dbPass = "test"
	)

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(dbName),
		tcpostgres.WithUsername(dbUser),
		tcpostgres.WithPassword(dbPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("pgtest: start container: %v", err)
	}

	t.Cleanup(func() {
		if err := ctr.Terminate(ctx); err != nil {
			t.Logf("pgtest: terminate container: %v", err)
		}
	})

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("pgtest: connection string: %v", err)
	}

	return connStr
}

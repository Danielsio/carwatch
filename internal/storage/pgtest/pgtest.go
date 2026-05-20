// Package pgtest provides a PostgreSQL-backed storage.Store for tests.
//
// If TEST_POSTGRES_DSN is set, it connects directly. Otherwise it spins up
// a throwaway PostgreSQL container via testcontainers.
package pgtest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/postgres"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationsDir returns the absolute path to the repo-level migrations/
// directory by navigating from this source file's location.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile is .../internal/storage/pgtest/pgtest.go
	// Navigate up 4 levels to the repo root, then into migrations/.
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations")
}

// tables lists every application table in deletion-safe order
// (children before parents to respect foreign-key constraints).
var tables = []string{
	"listing_user_seen", "pending_digest", "pending_notifications",
	"saved_listings", "hidden_listings", "listing_history",
	"seen_listings", "price_history", "link_tokens",
	"daily_digest", "searches", "price_list_cache",
	"push_subscriptions", "users",
}

// NewStore returns a storage.Store backed by PostgreSQL.
//
// When TEST_POSTGRES_DSN is set the store connects to that database directly.
// Otherwise a disposable PostgreSQL 16 container is started via
// testcontainers. In both cases migrations are applied automatically and
// t.Cleanup truncates all tables and closes the store.
func NewStore(t *testing.T) storage.Store {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = startContainer(t)
	}

	migrations := os.Getenv("TEST_POSTGRES_MIGRATIONS")
	if migrations == "" {
		migrations = migrationsDir()
	}

	store, err := postgres.New(dsn, migrations)
	if err != nil {
		t.Fatalf("pgtest: create store: %v", err)
	}

	t.Cleanup(func() {
		db := store.DB()
		for _, tbl := range tables {
			_, _ = db.Exec("DELETE FROM " + tbl)
		}
		_ = store.Close()
	})

	return store
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

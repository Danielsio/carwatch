package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestRun_InvalidConfigPath(t *testing.T) {
	err := run("/nonexistent/config.yaml", testLogger())
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestRun_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(path, []byte("invalid: {[broken yaml"), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestRun_MissingTelegramToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-token.yaml")
	cfg := `
log_level: info
polling:
  interval: 1m
  timezone: UTC
storage:
  db_path: ":memory:"
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error for missing telegram token")
	}
	if !strings.Contains(err.Error(), "telegram.token") {
		t.Errorf("expected telegram.token validation error, got: %v", err)
	}
}

func TestRun_InvalidDBPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := `
log_level: info
polling:
  interval: 1m
  timezone: UTC
telegram:
  token: "test-token"
storage:
  db_path: "/nonexistent/deep/path/db.sqlite"
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error for invalid DB path")
	}
	if !strings.Contains(err.Error(), "store") {
		t.Errorf("expected store creation error, got: %v", err)
	}
}

func TestRun_InvalidLogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := `
log_level: "invalid_level"
telegram:
  token: "test-token"
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if !strings.Contains(err.Error(), "log_level") {
		t.Errorf("expected log_level validation error, got: %v", err)
	}
}

func TestRun_InvalidActiveHours(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := `
log_level: info
polling:
  interval: 1m
  timezone: UTC
  active_hours:
    start: "not-a-time"
    end: "23:00"
telegram:
  token: "test-token"
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error for invalid active hours")
	}
	if !strings.Contains(err.Error(), "active_hours") {
		t.Errorf("expected active_hours validation error, got: %v", err)
	}
}

func TestRun_EmptyProxies_UsesDefault(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	path := filepath.Join(dir, "config.yaml")
	cfg := `
log_level: info
polling:
  interval: 1m
  timezone: UTC
telegram:
  token: "invalid-token-but-valid-format"
storage:
  db_path: "` + dbPath + `"
http:
  proxies: []
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	// Will fail at telegram connection, but should get past SQLite and fetcher creation.
	if err == nil {
		t.Fatal("expected error (telegram connection)")
	}
	if strings.Contains(err.Error(), "create store") || strings.Contains(err.Error(), "create fetcher") {
		t.Errorf("should not fail at store/fetcher creation, got: %v", err)
	}
}

func TestRun_WithProxies(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	path := filepath.Join(dir, "config.yaml")
	cfg := `
log_level: info
polling:
  interval: 1m
  timezone: UTC
telegram:
  token: "invalid-token-but-valid-format"
storage:
  db_path: "` + dbPath + `"
http:
  proxies:
    - "http://proxy1:8080"
    - "http://proxy2:8080"
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error (telegram connection)")
	}
	if strings.Contains(err.Error(), "create store") || strings.Contains(err.Error(), "create fetcher") {
		t.Errorf("should not fail at store/fetcher creation with proxies, got: %v", err)
	}
}

func TestRun_CustomLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			cfg := `
log_level: ` + level + `
telegram:
  token: "test-token"
storage:
  db_path: "/nonexistent/db.sqlite"
`
			_ = os.WriteFile(path, []byte(cfg), 0644)

			err := run(path, testLogger())
			if err == nil {
				t.Fatal("expected error (bad db path)")
			}
			// Should fail at store creation, not config/log level.
			if strings.Contains(err.Error(), "log_level") {
				t.Errorf("log_level %q should be valid, got: %v", level, err)
			}
		})
	}
}

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
  dsn: "postgres://localhost/test"
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

func TestRun_MissingDSN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := `
log_level: info
polling:
  interval: 1m
  timezone: UTC
telegram:
  token: "test-token"
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error for missing DSN")
	}
	if !strings.Contains(err.Error(), "storage.dsn is required") {
		t.Errorf("expected DSN validation error, got: %v", err)
	}
}

func TestRun_InvalidDSN(t *testing.T) {
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
  dsn: "postgres://localhost:99999/nonexistent_db?sslmode=disable&connect_timeout=1"
`
	_ = os.WriteFile(path, []byte(cfg), 0644)

	err := run(path, testLogger())
	if err == nil {
		t.Fatal("expected error for invalid DSN")
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
storage:
  dsn: "postgres://localhost/test"
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
storage:
  dsn: "postgres://localhost/test"
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
  dsn: "postgres://localhost:99999/nonexistent?sslmode=disable&connect_timeout=1"
`
			_ = os.WriteFile(path, []byte(cfg), 0644)

			err := run(path, testLogger())
			if err == nil {
				t.Fatal("expected error (bad DSN)")
			}
			if strings.Contains(err.Error(), "log_level") {
				t.Errorf("log_level %q should be valid, got: %v", level, err)
			}
		})
	}
}

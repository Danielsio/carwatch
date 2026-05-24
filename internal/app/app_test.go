package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher"
)

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("invalid: {[broken yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadConfig_ValidConfig(t *testing.T) {
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
  dsn: "postgres://localhost/test"
`
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Telegram.Token != "test-token" {
		t.Errorf("unexpected token: %s", c.Telegram.Token)
	}
}

func TestOpenStore_InvalidDSN(t *testing.T) {
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
  dsn: "postgres://localhost:99999/nonexistent?sslmode=disable&connect_timeout=1"
`
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = OpenStore(c)
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfgYAML := `
log_level: "invalid_level"
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
`
	if err := os.WriteFile(path, []byte(cfgYAML), 0644); err != nil {
		t.Fatal(err)
	}
	// Use config.Load directly since LoadConfig validates before returning
	// and log_level is validated inside config.Load.
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if !strings.Contains(err.Error(), "log_level") {
		t.Errorf("expected log_level error, got: %v", err)
	}
}

func TestSetupLogger_ValidLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			cfgYAML := `
log_level: ` + level + `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
`
			if err := os.WriteFile(path, []byte(cfgYAML), 0644); err != nil {
				t.Fatal(err)
			}
			c, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("log level %q should be valid, got: %v", level, err)
			}
			logger, levelVar, err := SetupLogger(c)
			if err != nil {
				t.Fatalf("SetupLogger failed for level %q: %v", level, err)
			}
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
			if levelVar == nil {
				t.Fatal("expected non-nil levelVar")
			}
		})
	}
}

func TestNewLogHandler_Auto(t *testing.T) {
	handler := NewLogHandler("auto", slog.LevelInfo)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewLogHandler_JSON(t *testing.T) {
	handler := NewLogHandler("json", slog.LevelDebug)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewLogHandler_Pretty(t *testing.T) {
	handler := NewLogHandler("pretty", slog.LevelWarn)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestBuildFetchers_NoProxy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfgYAML := `
log_level: info
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
`
	if err := os.WriteFile(path, []byte(cfgYAML), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(NewLogHandler("json", slog.LevelError))
	fb, err := BuildFetchers(c, logger)
	if err != nil {
		t.Fatal(err)
	}
	if fb.Yad2 == nil {
		t.Error("expected non-nil Yad2 fetcher")
	}
	if fb.Caching == nil {
		t.Error("expected non-nil Caching fetcher")
	}
	if fb.Factory == nil {
		t.Error("expected non-nil Factory")
	}
	if fb.Pool != nil {
		t.Error("expected nil Pool when no proxies configured")
	}
}

func TestBuildFetchers_CachingIsCircuitBreakerWrapped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfgYAML := `
log_level: info
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
`
	if err := os.WriteFile(path, []byte(cfgYAML), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(NewLogHandler("json", slog.LevelError))
	fb, err := BuildFetchers(c, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fb.Caching.(*fetcher.CircuitBreaker); !ok {
		t.Errorf("FetcherBundle.Caching should be *fetcher.CircuitBreaker, got %T", fb.Caching)
	}
}

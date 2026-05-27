package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiinternal "github.com/dsionov/carwatch/internal/api"
	"github.com/dsionov/carwatch/internal/config"
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

func TestBuildAPIRejectsMissingFirebaseOnNonLocalBind(t *testing.T) {
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Bind: "0.0.0.0:8080",
		},
		Telemetry: config.TelemetryConfig{
			AuthToken: "telemetry-secret",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	apiServer, err := BuildAPI(cfg, nil, nil, logger, nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected error, got nil (server=%v)", apiServer)
	}
	if !strings.Contains(err.Error(), "firebase auth must be configured") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "0.0.0.0:8080") {
		t.Fatalf("error should include bind address, got: %v", err)
	}
}

func TestBuildAPIFirebaseRequirementByBind(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name    string
		bind    string
		wantErr bool
	}{
		{name: "loopback v4", bind: "127.0.0.1:8080", wantErr: false},
		{name: "localhost", bind: "localhost:8080", wantErr: false},
		{name: "loopback v6", bind: "[::1]:8080", wantErr: false},
		{name: "all interfaces short", bind: ":8080", wantErr: true},
		{name: "all interfaces v4", bind: "0.0.0.0:8080", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				HTTP: config.HTTPConfig{
					Bind: tt.bind,
				},
				Telemetry: config.TelemetryConfig{
					AuthToken: "telemetry-secret",
				},
			}
			apiServer, err := BuildAPI(cfg, nil, nil, logger, nil, nil, nil, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (server=%v)", apiServer)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if apiServer == nil {
				t.Fatalf("expected api server, got nil")
			}
		})
	}
}

func TestBuildAPIRejectsMissingTelemetryAuthOnNonLocalBind(t *testing.T) {
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Bind: "0.0.0.0:8080",
		},
		Telemetry: config.TelemetryConfig{},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	apiServer, err := BuildAPI(cfg, nil, nil, logger, nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected error, got nil (server=%v)", apiServer)
	}
	if !strings.Contains(err.Error(), "telemetry.auth_token must be configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsNonLocalBind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		bind string
		want bool
	}{
		{name: "localhost", bind: "localhost:8080", want: false},
		{name: "loopback", bind: "127.0.0.1:8080", want: false},
		{name: "ipv6 loopback", bind: "[::1]:8080", want: false},
		{name: "empty", bind: "", want: false},
		{name: "whitespace", bind: "   ", want: false},
		{name: "all interfaces short", bind: ":8080", want: true},
		{name: "all interfaces v4", bind: "0.0.0.0:8080", want: true},
		{name: "public host", bind: "192.168.1.10:8080", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apiinternal.IsNonLocalBind(tt.bind); got != tt.want {
				t.Fatalf("IsNonLocalBind(%q) = %v, want %v", tt.bind, got, tt.want)
			}
		})
	}
}

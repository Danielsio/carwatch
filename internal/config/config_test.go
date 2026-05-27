package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
`
	cfg := loadFromString(t, yaml)

	if cfg.Telegram.Token != "test-token" {
		t.Errorf("token = %q", cfg.Telegram.Token)
	}
}

func TestLoad_Defaults(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
`
	cfg := loadFromString(t, yaml)

	if cfg.Polling.Interval.Minutes() != 15 {
		t.Errorf("default interval = %v", cfg.Polling.Interval)
	}
	if cfg.Polling.Jitter.Minutes() != 5 {
		t.Errorf("default jitter = %v", cfg.Polling.Jitter)
	}
	if cfg.Polling.Timezone != "Asia/Jerusalem" {
		t.Errorf("default timezone = %q", cfg.Polling.Timezone)
	}
	if cfg.Storage.Driver != "postgres" {
		t.Errorf("default driver = %q, want postgres", cfg.Storage.Driver)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default log_level = %q", cfg.LogLevel)
	}
	if len(cfg.HTTP.UserAgents) == 0 {
		t.Error("expected default user agents")
	}
	if cfg.Telegram.MaxSearches != 10 {
		t.Errorf("default max_searches = %d, want 10", cfg.Telegram.MaxSearches)
	}
}

func TestLoad_MaxSearchesExplicit(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
  max_searches: 5
storage:
  dsn: "postgres://localhost/test"
`
	cfg := loadFromString(t, yaml)

	if cfg.Telegram.MaxSearches != 5 {
		t.Errorf("max_searches = %d, want 5", cfg.Telegram.MaxSearches)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	yaml := `
polling:
  interval: 10m
storage:
  dsn: "postgres://localhost/test"
`
	expectLoadError(t, yaml, "telegram.token is required")
}

func TestLoad_InvalidActiveHours(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
polling:
  active_hours:
    start: "8am"
    end: "22:00"
`
	expectLoadError(t, yaml, "HH:MM")
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
log_level: verbose
`
	expectLoadError(t, yaml, "log_level")
}

func TestLoad_WarnHardcodedToken(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	yaml := `
telegram:
  token: "123456:ABCdef"
storage:
  dsn: "postgres://localhost/test"
`
	_ = loadFromString(t, yaml)
	if !strings.Contains(buf.String(), "telegram.token appears hardcoded") {
		t.Fatal("expected hardcoded token warning")
	}
}

func TestLoad_EnvVarTokenNoWarning(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")

	yaml := `
telegram:
  token: "${TELEGRAM_BOT_TOKEN}"
storage:
  dsn: "postgres://localhost/test"
`
	cfg := loadFromString(t, yaml)
	if cfg.Telegram.Token != "test-token" {
		t.Errorf("token = %q, want test-token", cfg.Telegram.Token)
	}
	if strings.Contains(buf.String(), "telegram.token appears hardcoded") {
		t.Fatal("did not expect hardcoded token warning for env-var token")
	}
}

func TestLoad_EnvVarInterpolation(t *testing.T) {
	t.Setenv("TEST_PROXY_URL", "socks5://proxy:1080")

	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
http:
  proxy: "${TEST_PROXY_URL}"
`
	cfg := loadFromString(t, yaml)

	if cfg.HTTP.Proxy != "socks5://proxy:1080" {
		t.Errorf("proxy = %q, want socks5://proxy:1080", cfg.HTTP.Proxy)
	}
}

func TestLoad_EnvVarInterpolationRedisPassword(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "redis-secret")

	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
redis:
  addr: "localhost:6379"
  password: "${REDIS_PASSWORD}"
`
	cfg := loadFromString(t, yaml)

	if cfg.Redis.Password != "redis-secret" {
		t.Errorf("redis.password = %q, want redis-secret", cfg.Redis.Password)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
		err   bool
	}{
		{"debug", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLogLevel(tt.input)
			if tt.err && err == nil {
				t.Error("expected error")
			}
			if !tt.err && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.err && got != tt.want {
				t.Errorf("level = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_InvalidLogFormat(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
log_format: xml
`
	expectLoadError(t, yaml, "log_format")
}

func TestLoad_InvalidHTTPBind(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
http:
  bind: "not-a-valid-address"
`
	expectLoadError(t, yaml, "http.bind")
}

func TestLoad_ValidActiveHours(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
polling:
  active_hours:
    start: "08:00"
    end: "22:00"
`
	cfg := loadFromString(t, yaml)

	if cfg.Polling.ActiveHours == nil {
		t.Fatal("expected active_hours to be set")
	}
	if cfg.Polling.ActiveHours.Start != "08:00" {
		t.Errorf("start = %q, want '08:00'", cfg.Polling.ActiveHours.Start)
	}
	if cfg.Polling.ActiveHours.End != "22:00" {
		t.Errorf("end = %q, want '22:00'", cfg.Polling.ActiveHours.End)
	}
}

func TestLoad_NegativeInterval(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
polling:
  interval: -5m
`
	expectLoadError(t, yaml, "polling.interval must be positive")
}

func TestLoad_NegativeJitter(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
polling:
  jitter: -1m
`
	expectLoadError(t, yaml, "polling.jitter must be non-negative")
}

func TestLoad_TelemetryDefaults(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
`
	cfg := loadFromString(t, yaml)

	if cfg.Telemetry.TracesExporter != "none" {
		t.Errorf("default traces_exporter = %q, want none", cfg.Telemetry.TracesExporter)
	}
	if cfg.Telemetry.MetricsPath != "/metrics" {
		t.Errorf("default metrics_path = %q, want /metrics", cfg.Telemetry.MetricsPath)
	}
}

func TestLoad_TelemetryExplicit(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
telemetry:
  traces_exporter: stdout
  otlp_endpoint: "collector:4317"
  metrics_path: /prom
`
	cfg := loadFromString(t, yaml)

	if cfg.Telemetry.TracesExporter != "stdout" {
		t.Errorf("traces_exporter = %q, want stdout", cfg.Telemetry.TracesExporter)
	}
	if cfg.Telemetry.OTLPEndpoint != "collector:4317" {
		t.Errorf("otlp_endpoint = %q, want collector:4317", cfg.Telemetry.OTLPEndpoint)
	}
	if cfg.Telemetry.MetricsPath != "/prom" {
		t.Errorf("metrics_path = %q, want /prom", cfg.Telemetry.MetricsPath)
	}
}

func TestLoad_TelemetryInvalidExporter(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
telemetry:
  traces_exporter: kafka
`
	expectLoadError(t, yaml, "telemetry.traces_exporter")
}

func TestLoad_NegativeRedisDB(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost/test"
redis:
  addr: "localhost:6379"
  db: -1
`
	expectLoadError(t, yaml, "redis.db must be >= 0")
}

func loadFromString(t *testing.T, yaml string) *Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func expectLoadError(t *testing.T, yaml string, contains string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsStr(err.Error(), contains) {
		t.Errorf("error %q should contain %q", err.Error(), contains)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

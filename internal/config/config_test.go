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
	if cfg.Storage.DBPath != "./data/dedup.db" {
		t.Errorf("default db_path = %q", cfg.Storage.DBPath)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default log_level = %q", cfg.LogLevel)
	}
	if len(cfg.HTTP.UserAgents) == 0 {
		t.Error("expected default user agents")
	}
	if cfg.Telegram.MaxSearches != 3 {
		t.Errorf("default max_searches = %d, want 3", cfg.Telegram.MaxSearches)
	}
}

func TestLoad_MaxSearchesExplicit(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
  max_searches: 5
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
`
	expectLoadError(t, yaml, "telegram.token is required")
}

func TestLoad_InvalidActiveHours(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
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
http:
  proxy: "${TEST_PROXY_URL}"
`
	cfg := loadFromString(t, yaml)

	if cfg.HTTP.Proxy != "socks5://proxy:1080" {
		t.Errorf("proxy = %q, want socks5://proxy:1080", cfg.HTTP.Proxy)
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
log_format: xml
`
	expectLoadError(t, yaml, "log_format")
}

func TestLoad_InvalidHTTPBind(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
http:
  bind: "not-a-valid-address"
`
	expectLoadError(t, yaml, "http.bind")
}

func TestLoad_ValidActiveHours(t *testing.T) {
	yaml := `
telegram:
  token: "test-token"
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

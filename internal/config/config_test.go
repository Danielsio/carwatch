package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	yaml := `
searches:
  - name: "test"
    source: yad2
    params:
      manufacturer: 27
    filters:
      engine_min_cc: 1800
    recipients:
      - "+972123456789"
`
	cfg := loadFromString(t, yaml)

	if len(cfg.Searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(cfg.Searches))
	}
	if cfg.Searches[0].Name != "test" {
		t.Errorf("name = %q", cfg.Searches[0].Name)
	}
	if cfg.Searches[0].Filters.EngineMinCC != 1800 {
		t.Errorf("engine_min_cc = %f", cfg.Searches[0].Filters.EngineMinCC)
	}
}

func TestLoad_Defaults(t *testing.T) {
	yaml := `
searches:
  - name: "test"
    source: yad2
    recipients:
      - "+972123456789"
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
  max_searches: 5
searches:
  - name: "test"
    source: yad2
    recipients:
      - "+972123456789"
`
	cfg := loadFromString(t, yaml)

	if cfg.Telegram.MaxSearches != 5 {
		t.Errorf("max_searches = %d, want 5", cfg.Telegram.MaxSearches)
	}
}

func TestLoad_NoSearches(t *testing.T) {
	yaml := `
polling:
  interval: 10m
`
	expectLoadError(t, yaml, "at least one search")
}

func TestLoad_MissingName(t *testing.T) {
	yaml := `
searches:
  - source: yad2
    recipients:
      - "+972123456789"
`
	expectLoadError(t, yaml, "name is required")
}

func TestLoad_MissingSource(t *testing.T) {
	yaml := `
searches:
  - name: "test"
    recipients:
      - "+972123456789"
`
	expectLoadError(t, yaml, "source is required")
}

func TestLoad_MissingRecipients(t *testing.T) {
	yaml := `
searches:
  - name: "test"
    source: yad2
`
	expectLoadError(t, yaml, "at least one recipient")
}

func TestLoad_InvalidActiveHours(t *testing.T) {
	yaml := `
polling:
  active_hours:
    start: "8am"
    end: "22:00"
searches:
  - name: "test"
    source: yad2
    recipients:
      - "+972123456789"
`
	expectLoadError(t, yaml, "HH:MM")
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	yaml := `
log_level: verbose
searches:
  - name: "test"
    source: yad2
    recipients:
      - "+972123456789"
`
	expectLoadError(t, yaml, "log_level")
}

func TestLoad_EngineLitersWarning(t *testing.T) {
	yaml := `
searches:
  - name: "test"
    source: yad2
    filters:
      engine_min_cc: 1.8
    recipients:
      - "+972123456789"
`
	expectLoadError(t, yaml, "looks like liters")
}

func TestLoad_EnvVarInterpolation(t *testing.T) {
	t.Setenv("TEST_PROXY_URL", "socks5://proxy:1080")

	yaml := `
http:
  proxy: "${TEST_PROXY_URL}"
searches:
  - name: "test"
    source: yad2
    recipients:
      - "+972123456789"
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

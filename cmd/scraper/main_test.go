package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_FailsWithoutRedisAddr(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfgYAML := `
log_level: info
telegram:
  token: "test-token"
storage:
  dsn: "postgres://localhost:5432/test?connect_timeout=1"
`
	if err := os.WriteFile(path, []byte(cfgYAML), 0644); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	err := run(path, "0.0.0.0:0", logger)
	if err == nil {
		t.Fatal("expected error when redis.addr is empty")
	}
	if !strings.Contains(err.Error(), "redis.addr is required") {
		t.Errorf("expected redis.addr error, got: %v", err)
	}
}

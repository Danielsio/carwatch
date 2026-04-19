package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_InvalidConfigPath(t *testing.T) {
	err := run("/nonexistent/config.yaml", nil)
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestRun_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(path, []byte("invalid: {[broken yaml"), 0644)

	err := run(path, nil)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestRun_NoSearches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	_ = os.WriteFile(path, []byte("log_level: info\n"), 0644)

	err := run(path, nil)
	if err == nil {
		t.Fatal("expected error for config with no searches")
	}
}

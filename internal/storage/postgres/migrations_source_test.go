package postgres

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// TestMigrationsSourceOpens guards cross-platform loading of the migrations
// directory. The previous "file://" source driver could not open a Windows
// `C:\…` path — it parsed as an invalid port, then as an unopenable path —
// which broke `make dev` on Windows. The iofs + os.DirFS source must open the
// real migrations dir and expose at least one migration on any OS.
func TestMigrationsSourceOpens(t *testing.T) {
	absPath, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	src, err := iofs.New(os.DirFS(absPath), ".")
	if err != nil {
		t.Fatalf("open migrations source at %q: %v", absPath, err)
	}
	defer func() { _ = src.Close() }()
	if _, err := src.First(); err != nil {
		t.Fatalf("expected at least one migration in %q: %v", absPath, err)
	}
}

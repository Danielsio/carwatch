package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db     *sql.DB
	dbPath string
}

func New(dbPath string) (*Store, error) {
	if dbPath != ":memory:" && !strings.HasPrefix(dbPath, "file::memory:") {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	db, err := sql.Open("sqlite3", dbPath+sep+"_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db, dbPath: dbPath}, nil
}

func (s *Store) Checkpoint() error {
	_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func (s *Store) Close() error {
	cpErr := s.Checkpoint()
	closeErr := s.db.Close()
	if cpErr != nil && closeErr != nil {
		return fmt.Errorf("checkpoint: %v; close db: %w", cpErr, closeErr)
	}
	if cpErr != nil {
		return fmt.Errorf("checkpoint: %w", cpErr)
	}
	return closeErr
}

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS seen_listings (
			token TEXT PRIMARY KEY,
			search_name TEXT NOT NULL,
			first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (s *Store) ClaimNew(ctx context.Context, token string, searchName string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO seen_listings (token, search_name) VALUES (?, ?)",
		token, searchName)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (s *Store) ReleaseClaim(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM seen_listings WHERE token = ?", token)
	return err
}

func (s *Store) Prune(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM seen_listings WHERE first_seen_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) Close() error {
	return s.db.Close()
}

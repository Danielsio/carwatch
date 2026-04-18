package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
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

func (s *Store) HasSeen(ctx context.Context, token string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM seen_listings WHERE token = ?)", token).Scan(&exists)
	return exists, err
}

func (s *Store) MarkSeen(ctx context.Context, token string, searchName string) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO seen_listings (token, search_name) VALUES (?, ?)", token, searchName)
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

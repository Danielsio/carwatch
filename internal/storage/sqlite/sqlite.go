package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/dsionov/carwatch/internal/storage"
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
		);
		CREATE TABLE IF NOT EXISTS pending_notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			recipient TEXT NOT NULL,
			search_name TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS price_history (
			token TEXT NOT NULL,
			price INTEGER NOT NULL,
			observed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (token, price)
		);
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

func (s *Store) EnqueueNotification(ctx context.Context, recipient, searchName, payload string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO pending_notifications (recipient, search_name, payload) VALUES (?, ?, ?)",
		recipient, searchName, payload)
	return err
}

func (s *Store) PendingNotifications(ctx context.Context) ([]storage.PendingNotification, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, recipient, search_name, payload FROM pending_notifications ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending []storage.PendingNotification
	for rows.Next() {
		var p storage.PendingNotification
		if err := rows.Scan(&p.ID, &p.Recipient, &p.SearchName, &p.Payload); err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

func (s *Store) AckNotification(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM pending_notifications WHERE id = ?", id)
	return err
}

func (s *Store) RecordPrice(ctx context.Context, token string, price int) (oldPrice int, changed bool, err error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT price FROM price_history WHERE token = ? ORDER BY observed_at DESC LIMIT 1", token)
	var prev int
	scanErr := row.Scan(&prev)

	_, err = s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO price_history (token, price) VALUES (?, ?)", token, price)
	if err != nil {
		return 0, false, err
	}

	if scanErr == sql.ErrNoRows {
		return 0, false, nil
	}
	if scanErr != nil {
		return 0, false, scanErr
	}

	if price < prev {
		return prev, true, nil
	}
	return prev, false, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

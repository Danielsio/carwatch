package postgres

import (
	"context"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) WriteCycleLog(ctx context.Context, entry storage.CycleLogEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cycle_log (started_at, duration_ms, searches, listings_fetched, listings_matched, notifications, error_message, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.StartedAt, entry.DurationMs, entry.Searches, entry.ListingsFetched,
		entry.ListingsMatched, entry.Notifications, entry.ErrorMessage, entry.Status)
	if err != nil {
		return fmt.Errorf("write cycle log: %w", err)
	}
	return nil
}

func (s *Store) ListCycleLogs(ctx context.Context, limit int) ([]storage.CycleLogEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, started_at, duration_ms, searches, listings_fetched, listings_matched, notifications, error_message, status
		FROM cycle_log
		ORDER BY started_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list cycle logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []storage.CycleLogEntry
	for rows.Next() {
		var e storage.CycleLogEntry
		if err := rows.Scan(&e.ID, &e.StartedAt, &e.DurationMs, &e.Searches,
			&e.ListingsFetched, &e.ListingsMatched, &e.Notifications,
			&e.ErrorMessage, &e.Status); err != nil {
			return nil, fmt.Errorf("scan cycle log: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

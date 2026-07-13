package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
)

// UpsertExtScanStatus records the extension's self-reported scan schedule and
// cycle stats. A cycle can arrive as several chunked pushes; chunks share the
// same started_at, so their stats accumulate instead of the last chunk
// winning. A report with a new started_at starts the counters over.
func (s *Store) UpsertExtScanStatus(ctx context.Context, st storage.ExtScanStatus) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ext_scan_status
			(chat_id, started_at, next_run_at, interval_sec, searches,
			 listings_fetched, listings_matched, notifications, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		ON CONFLICT (chat_id) DO UPDATE SET
			next_run_at  = EXCLUDED.next_run_at,
			interval_sec = EXCLUDED.interval_sec,
			searches     = EXCLUDED.searches,
			listings_fetched = CASE WHEN ext_scan_status.started_at = EXCLUDED.started_at
				THEN ext_scan_status.listings_fetched + EXCLUDED.listings_fetched
				ELSE EXCLUDED.listings_fetched END,
			listings_matched = CASE WHEN ext_scan_status.started_at = EXCLUDED.started_at
				THEN ext_scan_status.listings_matched + EXCLUDED.listings_matched
				ELSE EXCLUDED.listings_matched END,
			notifications = CASE WHEN ext_scan_status.started_at = EXCLUDED.started_at
				THEN ext_scan_status.notifications + EXCLUDED.notifications
				ELSE EXCLUDED.notifications END,
			started_at = EXCLUDED.started_at,
			updated_at = now()`,
		st.ChatID, st.StartedAt, st.NextRunAt, st.IntervalSec, st.Searches,
		st.ListingsFetched, st.ListingsMatched, st.Notifications)
	if err != nil {
		return fmt.Errorf("upsert ext scan status: %w", err)
	}
	return nil
}

// GetExtScanStatus returns the last reported extension scan status for the
// chat, or nil when the extension has never reported.
func (s *Store) GetExtScanStatus(ctx context.Context, chatID int64) (*storage.ExtScanStatus, error) {
	var st storage.ExtScanStatus
	err := s.db.QueryRowContext(ctx, `
		SELECT chat_id, started_at, next_run_at, interval_sec, searches,
		       listings_fetched, listings_matched, notifications, updated_at
		FROM ext_scan_status
		WHERE chat_id = $1`, chatID).Scan(
		&st.ChatID, &st.StartedAt, &st.NextRunAt, &st.IntervalSec, &st.Searches,
		&st.ListingsFetched, &st.ListingsMatched, &st.Notifications, &st.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ext scan status: %w", err)
	}
	return &st, nil
}

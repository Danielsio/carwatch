package sqlite

import (
	"context"
	"errors"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) NewListingsSince(ctx context.Context, chatID int64, since time.Time, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, sub_model, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, median_price, cohort_size, deal_score, first_seen_at
		FROM listing_history lh
		WHERE lh.chat_id = ? AND lh.first_seen_at > ?
		AND NOT EXISTS (
			SELECT 1 FROM listing_user_seen u
			WHERE u.chat_id = lh.chat_id AND u.token = lh.token
		)
		ORDER BY lh.first_seen_at DESC, lh.token DESC
		LIMIT ? OFFSET ?`, chatID, since, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		l, err := scanListingRow(rows)
		if err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func (s *Store) CountNewListingsSince(ctx context.Context, chatID int64, since time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM listing_history lh
		WHERE lh.chat_id = ? AND lh.first_seen_at > ?
		AND NOT EXISTS (
			SELECT 1 FROM listing_user_seen u
			WHERE u.chat_id = lh.chat_id AND u.token = lh.token
		)`,
		chatID, since).Scan(&count)
	return count, err
}

func (s *Store) GetLastSeenAt(ctx context.Context, chatID int64) (time.Time, error) {
	var raw string
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(last_seen_at, created_at) FROM users WHERE chat_id = ?",
		chatID).Scan(&raw)
	if err != nil {
		return time.Time{}, err
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid last_seen_at/created_at timestamp: " + raw)
}

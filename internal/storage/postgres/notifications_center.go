package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) NewListingsSince(ctx context.Context, chatID int64, since time.Time, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, sub_model, year, price,
			km, hand, city, page_link, image_url,
			engine_volume, horse_power, engine_type, gear_box, description,
			is_commercial, fitness_score, first_seen_at
		FROM listing_history lh
		WHERE lh.chat_id = $1 AND lh.first_seen_at > $2
		AND NOT EXISTS (
			SELECT 1 FROM listing_user_seen u
			WHERE u.chat_id = lh.chat_id AND u.token = lh.token
		)
		ORDER BY lh.first_seen_at DESC, lh.token DESC
		LIMIT $3 OFFSET $4`,
		chatID, since, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("new listings since: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		l, err := scanListingRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listing record: %w", err)
		}
		listings = append(listings, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("new listings since rows: %w", err)
	}
	return listings, nil
}

func (s *Store) CountNewListingsSince(ctx context.Context, chatID int64, since time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM listing_history lh
		WHERE lh.chat_id = $1 AND lh.first_seen_at > $2
		AND NOT EXISTS (
			SELECT 1 FROM listing_user_seen u
			WHERE u.chat_id = lh.chat_id AND u.token = lh.token
		)`,
		chatID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count new listings since: %w", err)
	}
	return count, nil
}

func (s *Store) GetLastSeenAt(ctx context.Context, chatID int64) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(last_seen_at, created_at) FROM users WHERE chat_id = $1`,
		chatID).Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("get last seen at: %w", err)
	}
	return t, nil
}

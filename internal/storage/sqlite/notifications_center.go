package sqlite

import (
	"context"
	"errors"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// notifFilterSQL applies each listing's parent search criteria so that
// notifications respect the user's current search settings (MaxKm,
// seller_filter, etc.) even if they were changed after the listing was saved.
const notifFilterSQL = `
	AND (s.max_km = 0 OR lh.km <= s.max_km OR lh.km = 0)
	AND (s.max_hand = 0 OR lh.hand <= s.max_hand)
	AND (s.price_min = 0 OR lh.price >= s.price_min OR lh.price = 0)
	AND (s.price_max = 0 OR lh.price <= s.price_max)
	AND (s.year_min = 0 OR lh.year >= s.year_min)
	AND (s.year_max = 0 OR lh.year <= s.year_max)
	AND (s.seller_filter = '' OR s.seller_filter = 'any'
		OR (s.seller_filter = 'private' AND lh.is_commercial = 0)
		OR (s.seller_filter = 'commercial' AND lh.is_commercial = 1))`

func (s *Store) NewListingsSince(ctx context.Context, chatID int64, since time.Time, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT lh.token, lh.search_name, lh.manufacturer, lh.model, lh.sub_model, lh.sub_model_id, lh.year, lh.price,
			lh.km, lh.hand, lh.city, lh.page_link, lh.image_url,
			lh.engine_volume, lh.horse_power, lh.engine_type, lh.gear_box, lh.description,
			lh.is_commercial, lh.fitness_score, lh.median_price, lh.cohort_size, lh.deal_score, lh.base_price, lh.first_seen_at, lh.removed_at
		FROM listing_history lh
		JOIN searches s ON s.id = lh.search_id AND s.chat_id = lh.chat_id
		WHERE lh.chat_id = ? AND lh.first_seen_at > ?
		AND NOT EXISTS (
			SELECT 1 FROM listing_user_seen u
			WHERE u.chat_id = lh.chat_id AND u.token = lh.token
		)`+notifFilterSQL+`
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
		JOIN searches s ON s.id = lh.search_id AND s.chat_id = lh.chat_id
		WHERE lh.chat_id = ? AND lh.first_seen_at > ?
		AND NOT EXISTS (
			SELECT 1 FROM listing_user_seen u
			WHERE u.chat_id = lh.chat_id AND u.token = lh.token
		)`+notifFilterSQL,
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

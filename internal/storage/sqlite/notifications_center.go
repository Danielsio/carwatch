package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) NewListingsSince(ctx context.Context, chatID int64, since time.Time, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE chat_id = ? AND first_seen_at > ?
		ORDER BY first_seen_at DESC, token DESC
		LIMIT ? OFFSET ?`, chatID, since, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		var l storage.ListingRecord
		var fs sql.NullFloat64
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt); err != nil {
			return nil, err
		}
		if fs.Valid {
			l.FitnessScore = &fs.Float64
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func (s *Store) CountNewListingsSince(ctx context.Context, chatID int64, since time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM listing_history WHERE chat_id = ? AND first_seen_at > ?",
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

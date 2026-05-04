package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) NewListingsSince(ctx context.Context, chatID int64, since time.Time, limit, offset int) ([]storage.ListingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		WHERE chat_id = $1 AND first_seen_at > $2
		ORDER BY first_seen_at DESC, token DESC
		LIMIT $3 OFFSET $4`,
		chatID, since, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("new listings since: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.ListingRecord
	for rows.Next() {
		var l storage.ListingRecord
		var fs sql.NullFloat64
		if err := rows.Scan(&l.Token, &l.SearchName, &l.Manufacturer, &l.Model,
			&l.Year, &l.Price, &l.Km, &l.Hand, &l.City, &l.PageLink, &l.ImageURL, &fs, &l.FirstSeenAt); err != nil {
			return nil, fmt.Errorf("scan listing record: %w", err)
		}
		if fs.Valid {
			l.FitnessScore = &fs.Float64
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
		`SELECT COUNT(*) FROM listing_history WHERE chat_id = $1 AND first_seen_at > $2`,
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

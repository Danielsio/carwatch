package sqlite

import (
	"context"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) MarketListings(ctx context.Context) ([]storage.MarketListing, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT lh.manufacturer, lh.model, lh.year, lh.price, lh.km
		FROM listing_history lh
		INNER JOIN (
			SELECT token, MAX(first_seen_at) AS max_seen
			FROM listing_history
			GROUP BY token
		) latest ON lh.token = latest.token AND lh.first_seen_at = latest.max_seen
		WHERE lh.manufacturer IS NOT NULL AND lh.manufacturer != ''
		  AND lh.model IS NOT NULL AND lh.model != ''
		  AND lh.year > 0 AND lh.price > 0`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.MarketListing
	for rows.Next() {
		var l storage.MarketListing
		if err := rows.Scan(&l.Manufacturer, &l.Model, &l.Year, &l.Price, &l.Km); err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

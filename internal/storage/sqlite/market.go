package sqlite

import (
	"context"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) MarketListings(ctx context.Context) ([]storage.MarketListing, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT manufacturer, model, year, price
		FROM market_cache
		WHERE manufacturer != '' AND model != ''
		  AND year > 0 AND price > 0`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var listings []storage.MarketListing
	for rows.Next() {
		var l storage.MarketListing
		if err := rows.Scan(&l.Manufacturer, &l.Model, &l.Year, &l.Price); err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

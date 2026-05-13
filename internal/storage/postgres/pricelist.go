package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) GetPriceListEntry(ctx context.Context, subModelID, year int) (*storage.PriceListEntry, error) {
	var e storage.PriceListEntry
	err := s.db.QueryRowContext(ctx,
		`SELECT sub_model_id, year, base_price, title, fetched_at
		 FROM price_list_cache
		 WHERE sub_model_id = $1 AND year = $2`, subModelID, year,
	).Scan(&e.SubModelID, &e.Year, &e.BasePrice, &e.Title, &e.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get price list entry: %w", err)
	}
	return &e, nil
}

func (s *Store) SetPriceListEntry(ctx context.Context, e storage.PriceListEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO price_list_cache (sub_model_id, year, base_price, title, fetched_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (sub_model_id, year) DO UPDATE SET
			base_price = EXCLUDED.base_price,
			title = EXCLUDED.title,
			fetched_at = NOW()`,
		e.SubModelID, e.Year, e.BasePrice, e.Title,
	)
	if err != nil {
		return fmt.Errorf("set price list entry: %w", err)
	}
	return nil
}

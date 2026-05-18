package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/timeutil"
)

func (s *Store) GetPriceListEntry(ctx context.Context, subModelID, year int) (*storage.PriceListEntry, error) {
	var e storage.PriceListEntry
	var fetchedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT sub_model_id, year, base_price, title, fetched_at
		 FROM price_list_cache
		 WHERE sub_model_id = ? AND year = ?`, subModelID, year,
	).Scan(&e.SubModelID, &e.Year, &e.BasePrice, &e.Title, &fetchedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get price list entry: %w", err)
	}
	e.FetchedAt, err = timeutil.ParseFlexible(fetchedAt)
	if err != nil {
		return nil, fmt.Errorf("parse fetched_at: %w", err)
	}
	return &e, nil
}

func (s *Store) SetPriceListEntry(ctx context.Context, e storage.PriceListEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO price_list_cache (sub_model_id, year, base_price, title, fetched_at)
		 VALUES (?, ?, ?, ?, datetime('now'))
		 ON CONFLICT (sub_model_id, year) DO UPDATE SET
			base_price = excluded.base_price,
			title = excluded.title,
			fetched_at = datetime('now')`,
		e.SubModelID, e.Year, e.BasePrice, e.Title,
	)
	if err != nil {
		return fmt.Errorf("set price list entry: %w", err)
	}
	return nil
}

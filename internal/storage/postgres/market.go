package postgres

import (
	"context"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) RefreshMarketMedians(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY market_medians")
	return err
}

func (s *Store) LoadMarketMedians(ctx context.Context) ([]storage.MarketMedianRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT manufacturer, model, year,
		       COALESCE(median_price, 0)::int,
		       COALESCE(median_km, 0)::int,
		       cohort_size
		FROM market_medians`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []storage.MarketMedianRow
	for rows.Next() {
		var r storage.MarketMedianRow
		if err := rows.Scan(&r.Manufacturer, &r.Model, &r.Year, &r.MedianPrice, &r.MedianKm, &r.CohortSize); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

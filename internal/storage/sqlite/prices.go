package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) RecordPrice(ctx context.Context, token string, price int) (oldPrice int, changed bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		"SELECT price FROM price_history WHERE token = ? ORDER BY observed_at DESC, rowid DESC LIMIT 1", token)
	var prev int
	scanErr := row.Scan(&prev)

	_, err = tx.ExecContext(ctx,
		"INSERT OR IGNORE INTO price_history (token, price) VALUES (?, ?)", token, price)
	if err != nil {
		return 0, false, err
	}

	if scanErr == sql.ErrNoRows {
		return 0, false, tx.Commit()
	}
	if scanErr != nil {
		return 0, false, scanErr
	}

	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	if price != prev {
		return prev, true, nil
	}
	return prev, false, nil
}

func (s *Store) GetPriceHistory(ctx context.Context, token string) ([]storage.PricePoint, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT price, observed_at FROM price_history WHERE token = ? ORDER BY observed_at DESC", token)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var points []storage.PricePoint
	for rows.Next() {
		var p storage.PricePoint
		if err := rows.Scan(&p.Price, &p.ObservedAt); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (s *Store) PrunePrices(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM price_history WHERE observed_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

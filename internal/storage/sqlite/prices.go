package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) RecordPrice(ctx context.Context, token string, price int) (oldPrice int, changed bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("record price begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		"SELECT price FROM price_history WHERE token = ? ORDER BY observed_at DESC, rowid DESC LIMIT 1", token)
	var prev int
	scanErr := row.Scan(&prev)

	_, err = tx.ExecContext(ctx,
		"INSERT INTO price_history (token, price) VALUES (?, ?)",
		token, price)
	if err != nil {
		return 0, false, fmt.Errorf("record price insert: %w", err)
	}

	if scanErr == sql.ErrNoRows {
		if err := tx.Commit(); err != nil {
			return 0, false, fmt.Errorf("record price commit: %w", err)
		}
		return 0, false, nil
	}
	if scanErr != nil {
		return 0, false, fmt.Errorf("record price scan prev: %w", scanErr)
	}

	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("record price commit: %w", err)
	}
	if price != prev {
		return prev, true, nil
	}
	return prev, false, nil
}

func (s *Store) GetPriceHistory(ctx context.Context, token string) ([]storage.PricePoint, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT price, observed_at FROM price_history WHERE token = ? ORDER BY observed_at DESC, rowid DESC", token)
	if err != nil {
		return nil, fmt.Errorf("get price history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var points []storage.PricePoint
	for rows.Next() {
		var p storage.PricePoint
		if err := rows.Scan(&p.Price, &p.ObservedAt); err != nil {
			return nil, fmt.Errorf("scan price point: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get price history rows: %w", err)
	}
	return points, nil
}

func (s *Store) PrunePrices(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM price_history WHERE observed_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune prices: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune prices rows affected: %w", err)
	}
	return n, nil
}

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// price_history is keyed by token alone and is fed by user-supplied ingest
// pushes, so it is the one shared table written from untrusted input: any user
// watching a car can push a price for it, and every user's price chart and
// drop-detection for that car reads the same rows. A fabricated price is
// therefore a cross-tenant poisoning vector — most damagingly a fake steep drop
// that fires a "price dropped!" alert, the product's core trust signal.
//
// implausiblePriceSwing rejects the high-impact version of that: an extreme
// single-step move that no real used-car listing makes. A price that jumps to
// several times the last observation, or collapses to a small fraction of it,
// is an error or manipulation, not a sale. Recording it would both corrupt the
// chart and, on the downward side, trigger a false drop alert. A skipped
// observation only means that one wild value is not charted; the listing's own
// displayed price is unaffected (that is guarded separately in the upsert), and
// a genuine price that is merely re-observed every cycle still lands the moment
// it falls inside the plausible band.
const (
	maxPlausiblePriceRatio = 3.0 // > 3x the previous price
	minPlausiblePriceRatio = 1.0 / 3.0
)

func implausiblePriceSwing(prev, next int) bool {
	if prev <= 0 || next <= 0 {
		return next <= 0 // a non-positive price is never a real observation
	}
	ratio := float64(next) / float64(prev)
	return ratio > maxPlausiblePriceRatio || ratio < minPlausiblePriceRatio
}

func (s *Store) RecordPrice(ctx context.Context, token string, price int) (oldPrice int, changed bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("record price begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		`SELECT price FROM price_history WHERE token = $1 ORDER BY observed_at DESC, id DESC LIMIT 1`,
		token)
	var prev int
	scanErr := row.Scan(&prev)

	if scanErr == sql.ErrNoRows {
		// First observation: insert only if it is a real, positive price. A
		// non-positive first observation would seed the whole history with junk.
		if price <= 0 {
			if err := tx.Commit(); err != nil {
				return 0, false, fmt.Errorf("record price commit: %w", err)
			}
			return 0, false, nil
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO price_history (token, price) VALUES ($1, $2)`,
			token, price); err != nil {
			return 0, false, fmt.Errorf("record price insert: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return 0, false, fmt.Errorf("record price commit: %w", err)
		}
		return 0, false, nil
	}
	if scanErr != nil {
		return 0, false, fmt.Errorf("record price scan prev: %w", scanErr)
	}

	// Refuse an implausible swing: it does not enter the history and, crucially,
	// is not reported as a change — so it cannot fire a price-drop alert.
	if price != prev && implausiblePriceSwing(prev, price) {
		if err := tx.Commit(); err != nil {
			return 0, false, fmt.Errorf("record price commit: %w", err)
		}
		return prev, false, nil
	}

	if price != prev {
		// Price changed: insert a new row.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO price_history (token, price) VALUES ($1, $2)`,
			token, price); err != nil {
			return 0, false, fmt.Errorf("record price insert: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return 0, false, fmt.Errorf("record price commit: %w", err)
		}
		return prev, true, nil
	}

	// Price unchanged: no insert needed.
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("record price commit: %w", err)
	}
	return prev, false, nil
}

func (s *Store) PeekPrice(ctx context.Context, token string) (int, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT price FROM price_history WHERE token = $1 ORDER BY observed_at DESC, id DESC LIMIT 1`,
		token)
	var price int
	switch err := row.Scan(&price); {
	case err == sql.ErrNoRows:
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("peek price: %w", err)
	}
	return price, true, nil
}

func (s *Store) GetPriceHistory(ctx context.Context, token string) ([]storage.PricePoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT price, observed_at FROM price_history WHERE token = $1 ORDER BY observed_at DESC, id DESC`,
		token)
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
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM price_history WHERE observed_at < $1`,
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune prices: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune prices rows affected: %w", err)
	}
	return n, nil
}

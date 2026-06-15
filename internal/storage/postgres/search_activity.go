package postgres

import (
	"context"
	"fmt"

	"github.com/dsionov/carwatch/internal/storage"
)

// SearchDailyCounts returns the number of distinct listings first seen per day
// for a search over the last `days` days. Days with no new listings are omitted
// (callers fill gaps). `days` is clamped to a sane 1..90 window.
func (s *Store) SearchDailyCounts(ctx context.Context, chatID, searchID int64, days int) ([]storage.DailyListingCount, error) {
	if days <= 0 || days > 90 {
		days = 14
	}

	// Dense, 0-filled series over the last `days` days (server-local day
	// buckets) so the client never has to reconcile timezones when plotting.
	rows, err := s.db.QueryContext(ctx, `
		SELECT to_char(d.day, 'YYYY-MM-DD') AS day, COALESCE(c.cnt, 0)::int AS cnt
		FROM generate_series(
		       date_trunc('day', NOW()) - make_interval(days => $3 - 1),
		       date_trunc('day', NOW()),
		       interval '1 day'
		     ) AS d(day)
		LEFT JOIN (
		       SELECT date_trunc('day', first_seen_at) AS day, COUNT(DISTINCT token) AS cnt
		       FROM listing_history
		       WHERE chat_id = $1 AND search_id = $2
		         AND first_seen_at >= date_trunc('day', NOW()) - make_interval(days => $3 - 1)
		       GROUP BY 1
		     ) c ON c.day = d.day
		ORDER BY d.day`, chatID, searchID, days)
	if err != nil {
		return nil, fmt.Errorf("search daily counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []storage.DailyListingCount
	for rows.Next() {
		var d storage.DailyListingCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, fmt.Errorf("scan daily count: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

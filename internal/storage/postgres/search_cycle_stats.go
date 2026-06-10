package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) UpsertSearchCycleStats(ctx context.Context, stats []storage.SearchCycleStats) error {
	if len(stats) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(`INSERT INTO search_cycle_stats (search_id, chat_id, search_name, cycle_at, feed_size, matched, new_listings, km_filtered, delivered, price_drops, updated_at)
VALUES `)

	args := make([]any, 0, len(stats)*10)
	for i, st := range stats {
		if i > 0 {
			b.WriteString(", ")
		}
		base := i * 10
		fmt.Fprintf(&b, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,NOW())",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10)
		args = append(args, st.SearchID, st.ChatID, st.SearchName, st.CycleAt,
			st.FeedSize, st.Matched, st.NewListings, st.KmFiltered, st.Delivered, st.PriceDrops)
	}

	b.WriteString(` ON CONFLICT (search_id) DO UPDATE SET
		chat_id = EXCLUDED.chat_id,
		search_name = EXCLUDED.search_name,
		cycle_at = EXCLUDED.cycle_at,
		feed_size = EXCLUDED.feed_size,
		matched = EXCLUDED.matched,
		new_listings = EXCLUDED.new_listings,
		km_filtered = EXCLUDED.km_filtered,
		delivered = EXCLUDED.delivered,
		price_drops = EXCLUDED.price_drops,
		updated_at = NOW()`)

	_, err := s.db.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return fmt.Errorf("upsert search cycle stats: %w", err)
	}
	return nil
}

func (s *Store) ListSearchCycleStats(ctx context.Context, chatID int64) ([]storage.SearchCycleStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT search_id, chat_id, search_name, cycle_at, feed_size, matched, new_listings, km_filtered, delivered, price_drops
		FROM search_cycle_stats
		WHERE chat_id = $1
		ORDER BY search_name`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list search cycle stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []storage.SearchCycleStats
	for rows.Next() {
		var st storage.SearchCycleStats
		if err := rows.Scan(&st.SearchID, &st.ChatID, &st.SearchName, &st.CycleAt,
			&st.FeedSize, &st.Matched, &st.NewListings, &st.KmFiltered, &st.Delivered, &st.PriceDrops); err != nil {
			return nil, fmt.Errorf("scan search cycle stats: %w", err)
		}
		stats = append(stats, st)
	}
	return stats, rows.Err()
}

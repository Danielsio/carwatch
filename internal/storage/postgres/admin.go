package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) DBFileSize() (int64, error) {
	var size int64
	err := s.db.QueryRowContext(context.Background(), `SELECT pg_database_size(current_database())`).Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("pg_database_size: %w", err)
	}
	return size, nil
}

func (s *Store) CountAllListings(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listing_history`).Scan(&count)
	return count, err
}

func (s *Store) TableSizes(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}

	sizes := make(map[string]int64, len(tables))
	for _, t := range tables {
		var count int64
		q := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteIdent(t))
		if err := s.db.QueryRowContext(ctx, q).Scan(&count); err != nil {
			return nil, fmt.Errorf("count rows for table %s: %w", t, err)
		}
		sizes[t] = count
	}
	return sizes, nil
}

// quoteIdent returns a safely quoted PostgreSQL identifier (table name from information_schema).
func quoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

var purgeable = map[string]bool{
	"listing_history":         true,
	"price_history":           true,
	"seen_listings":           true,
	"pending_notifications":   true,
	"saved_listings":          true,
	"hidden_listings":         true,
	"pending_digest":          true,
}

func (s *Store) PurgeTable(ctx context.Context, table string) (int64, error) {
	if !purgeable[table] {
		return 0, fmt.Errorf("%w: %q", storage.ErrNotPurgeable, table)
	}
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, quoteIdent(table)))
	if err != nil {
		return 0, fmt.Errorf("purge %s: %w", table, err)
	}
	return result.RowsAffected()
}

func (s *Store) AdminListListings(ctx context.Context, limit, offset int) ([]storage.ListingRecord, int64, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listing_history`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT token, chat_id, search_id, search_name, manufacturer, model, year, price,
			km, hand, city, page_link, image_url, fitness_score, first_seen_at
		FROM listing_history
		ORDER BY first_seen_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query listings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []storage.ListingRecord
	for rows.Next() {
		var r storage.ListingRecord
		var fs sql.NullFloat64
		if err := rows.Scan(
			&r.Token, &r.ChatID, &r.SearchID, &r.SearchName,
			&r.Manufacturer, &r.Model, &r.Year, &r.Price,
			&r.Km, &r.Hand, &r.City, &r.PageLink, &r.ImageURL,
			&fs, &r.FirstSeenAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}
		if fs.Valid {
			r.FitnessScore = &fs.Float64
		}
		items = append(items, r)
	}
	return items, total, rows.Err()
}

func (s *Store) AdminDeleteListing(ctx context.Context, token string, chatID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM listing_history WHERE token = $1 AND chat_id = $2`, token, chatID)
	return err
}

func (s *Store) AdminListSearches(ctx context.Context) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max,
			price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys, active, created_at,
			COALESCE(share_token, '')
		FROM searches
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("admin list searches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanSearches(rows)
}

func (s *Store) AdminListUsers(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, username, state, state_data, created_at, active, language, tier,
			tier_expires_at, trial_used
		FROM users
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("admin list users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanUsers(rows)
}

func (s *Store) VacuumDB(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	return nil
}

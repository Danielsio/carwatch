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
	// Sum actual user-table sizes instead of pg_database_size(), which
	// includes ~5-8 MB of system catalog overhead that VACUUM cannot reclaim.
	err := s.db.QueryRowContext(context.Background(), `
		SELECT COALESCE(SUM(pg_total_relation_size(quote_ident(table_name))), 0)
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`).Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("pg user table size: %w", err)
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

func (s *Store) AdminListListings(ctx context.Context, limit, offset int, searchID int64) ([]storage.ListingRecord, int64, error) {
	var total int64
	if searchID > 0 {
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listing_history WHERE search_id = $1`, searchID).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count listings: %w", err)
		}
	} else {
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listing_history`).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count listings: %w", err)
		}
	}

	var rows *sql.Rows
	var err error
	if searchID > 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT token, chat_id, search_id, search_name, manufacturer, model, sub_model, year, price,
				km, hand, city, page_link, image_url,
				engine_volume, horse_power, engine_type, gear_box, description,
				fitness_score, first_seen_at
			FROM listing_history
			WHERE search_id = $1
			ORDER BY first_seen_at DESC
			LIMIT $2 OFFSET $3`, searchID, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT token, chat_id, search_id, search_name, manufacturer, model, sub_model, year, price,
				km, hand, city, page_link, image_url,
				engine_volume, horse_power, engine_type, gear_box, description,
				fitness_score, first_seen_at
			FROM listing_history
			ORDER BY first_seen_at DESC
			LIMIT $1 OFFSET $2`, limit, offset)
	}
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
			&r.Manufacturer, &r.Model, &r.SubModel, &r.Year, &r.Price,
			&r.Km, &r.Hand, &r.City, &r.PageLink, &r.ImageURL,
			&r.EngineVolume, &r.HorsePower, &r.EngineType, &r.GearBox, &r.Description,
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

func (s *Store) AdminDeleteSearch(ctx context.Context, id int64) error {
	var chatID int64
	err := s.db.QueryRowContext(ctx, `SELECT chat_id FROM searches WHERE id = $1`, id).Scan(&chatID)
	if err != nil {
		return fmt.Errorf("lookup search: %w", err)
	}
	return s.DeleteSearch(ctx, id, chatID)
}

func (s *Store) AdminListUsers(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, username, state, state_data, created_at, active, language, tier,
			tier_expires_at, trial_used, channel, channel_id
		FROM users
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("admin list users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAdminUsers(rows)
}

func scanAdminUsers(rows *sql.Rows) ([]storage.User, error) {
	var users []storage.User
	for rows.Next() {
		var u storage.User
		if err := rows.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
			&u.Tier, &u.TierExpires, &u.TrialUsed, &u.Channel, &u.ChannelID); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) AdminDeleteUser(ctx context.Context, chatID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("admin delete user begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range []string{
		"searches", "listing_history", "seen_listings",
		"saved_listings", "hidden_listings",
		"pending_digest",
	} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE chat_id = $1`, quoteIdent(table)), chatID); err != nil {
			return fmt.Errorf("admin delete user data from %s: %w", table, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE chat_id = $1`, chatID); err != nil {
		return fmt.Errorf("admin delete user: %w", err)
	}
	return tx.Commit()
}

func (s *Store) VacuumDB(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	return nil
}

func (s *Store) SyncUserActiveStatus(ctx context.Context) (activated, deactivated int64, err error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET active = true
		WHERE active = false
		  AND chat_id IN (SELECT DISTINCT chat_id FROM searches WHERE active = true)`)
	if err != nil {
		return 0, 0, fmt.Errorf("activate users with searches: %w", err)
	}
	activated, _ = res.RowsAffected()

	res, err = s.db.ExecContext(ctx, `
		UPDATE users SET active = false
		WHERE active = true
		  AND chat_id NOT IN (SELECT DISTINCT chat_id FROM searches WHERE active = true)`)
	if err != nil {
		return activated, 0, fmt.Errorf("deactivate users without searches: %w", err)
	}
	deactivated, _ = res.RowsAffected()

	return activated, deactivated, nil
}

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
		SELECT COALESCE(SUM(pg_total_relation_size(quote_ident(table_schema) || '.' || quote_ident(table_name))), 0)
		FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'`).Scan(&size)
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
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
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

// quoteIdent returns a safely quoted PostgreSQL identifier. It escapes quoting
// but performs NO validation: callers MUST only pass identifiers from a trusted
// source — information_schema query results, the `purgeable` allowlist, or the
// `resetTables` constant — never raw user input. It exists because table names
// cannot be passed as bind parameters.
func quoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

var purgeable = map[string]bool{
	"listing_history":   true,
	"price_history":     true,
	"seen_listings":     true,
	"listing_user_seen": true,
	"saved_listings":    true,
	"hidden_listings":   true,
	"pending_digest":    true,
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

var resetTables = []string{
	"search_cycle_stats",
	"listing_user_seen",
	"saved_listings",
	"hidden_listings",
	"seen_listings",
	"pending_digest",
	"listing_history",
	"price_history",
	"price_list_cache",
	"cycle_log",
	"link_tokens",
}

func (s *Store) ResetAllData(ctx context.Context) (map[string]int64, error) {
	counts := make(map[string]int64, len(resetTables))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reset begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range resetTables {
		var count int64
		if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(table))).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("TRUNCATE %s CASCADE", quoteIdent(table))); err != nil {
			return nil, fmt.Errorf("truncate %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reset commit: %w", err)
	}
	return counts, nil
}

func (s *Store) AdminListListings(ctx context.Context, limit, offset int, searchID, chatID int64) ([]storage.ListingRecord, int64, error) {
	var conds []string
	var filterArgs []any
	if searchID > 0 {
		filterArgs = append(filterArgs, searchID)
		conds = append(conds, fmt.Sprintf("lh.search_id = $%d", len(filterArgs)))
	}
	if chatID > 0 {
		filterArgs = append(filterArgs, chatID)
		conds = append(conds, fmt.Sprintf("lh.chat_id = $%d", len(filterArgs)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM listing_history lh`+where, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
	}

	args := append([]any{}, filterArgs...)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT lh.token, lh.chat_id, lh.search_id, lh.search_name, lh.manufacturer, lh.model, lh.sub_model, lh.sub_model_id, lh.year, lh.price,
			lh.km, lh.hand, lh.city, lh.page_link, lh.image_url,
			lh.engine_volume, lh.horse_power, lh.engine_type, lh.gear_box, lh.description,
			lh.is_commercial, lh.fitness_score, lh.median_price, lh.cohort_size, lh.deal_score, lh.base_price, lh.first_seen_at, lh.posted_at, lh.removed_at,
			d.status, d.alert_type, d.created_at
		FROM listing_history lh
		LEFT JOIN LATERAL (
			SELECT nd.status, nd.alert_type, nd.created_at
			FROM notification_deliveries nd
			WHERE nd.chat_id = lh.chat_id AND nd.token = lh.token
			ORDER BY nd.created_at DESC
			LIMIT 1
		) d ON true`+where+`
		ORDER BY lh.first_seen_at DESC
		LIMIT $`+fmt.Sprintf("%d", len(filterArgs)+1)+` OFFSET $`+fmt.Sprintf("%d", len(filterArgs)+2), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query listings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []storage.ListingRecord
	for rows.Next() {
		var r storage.ListingRecord
		var fs sql.NullFloat64
		var ic, mp, cs, ds, bp sql.NullInt64
		var postedAt, removedAt, notifiedAt sql.NullTime
		var notifyStatus, notifyVia sql.NullString
		if err := rows.Scan(
			&r.Token, &r.ChatID, &r.SearchID, &r.SearchName,
			&r.Manufacturer, &r.Model, &r.SubModel, &r.SubModelID, &r.Year, &r.Price,
			&r.Km, &r.Hand, &r.City, &r.PageLink, &r.ImageURL,
			&r.EngineVolume, &r.HorsePower, &r.EngineType, &r.GearBox, &r.Description,
			&ic, &fs, &mp, &cs, &ds, &bp, &r.FirstSeenAt, &postedAt, &removedAt,
			&notifyStatus, &notifyVia, &notifiedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}
		r.IsCommercial = storage.ListingCommercialFromSQL(ic)
		if notifiedAt.Valid {
			r.NotifiedAt = &notifiedAt.Time
		}
		r.NotifyStatus = notifyStatus.String
		r.NotifyVia = notifyVia.String
		if postedAt.Valid {
			r.PostedAt = &postedAt.Time
		}
		if fs.Valid {
			r.FitnessScore = &fs.Float64
		}
		if mp.Valid {
			v := int(mp.Int64)
			r.MedianPrice = &v
		}
		if cs.Valid {
			v := int(cs.Int64)
			r.CohortSize = &v
		}
		if ds.Valid {
			v := int(ds.Int64)
			r.DealScore = &v
		}
		if bp.Valid {
			v := int(bp.Int64)
			r.BasePrice = &v
		}
		if removedAt.Valid {
			r.RemovedAt = &removedAt.Time
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
			COALESCE(price_min, 0), price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys,
			COALESCE(seller_filter, 'any'), COALESCE(gear_box, ''), price_only, photo_only, active, created_at,
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

	// price_history is global (keyed by token, no chat_id) and shared
	// across users. PrunePrices handles cleanup via retention policy.

	for _, table := range []string{
		"searches", "listing_history", "seen_listings", "listing_user_seen",
		"saved_listings", "hidden_listings",
		"push_subscriptions", "pending_digest",
	} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE chat_id = $1`, quoteIdent(table)), chatID); err != nil {
			return fmt.Errorf("admin delete user data from %s: %w", table, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM link_tokens WHERE web_chat_id = $1`, chatID); err != nil {
		return fmt.Errorf("admin delete user link_tokens: %w", err)
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

func (s *Store) AdminActivityStats(ctx context.Context, days int) ([]storage.AdminDayActivity, error) {
	if days <= 0 {
		days = 30
	}

	rows, err := s.db.QueryContext(ctx, `
		WITH days AS (
			SELECT generate_series(
				CURRENT_DATE - $1 * interval '1 day',
				CURRENT_DATE,
				'1 day'::interval
			)::date AS day
		)
		SELECT d.day::text,
			COALESCE(l.cnt, 0),
			COALESCE(p.cnt, 0),
			COALESCE(u.cnt, 0)
		FROM days d
		LEFT JOIN (
			SELECT first_seen_at::date AS day, COUNT(*) AS cnt
			FROM listing_history
			WHERE first_seen_at >= CURRENT_DATE - $1 * interval '1 day'
			GROUP BY first_seen_at::date
		) l ON d.day = l.day
		LEFT JOIN (
			SELECT observed_at::date AS day, COUNT(*) AS cnt
			FROM price_history
			WHERE observed_at >= CURRENT_DATE - $1 * interval '1 day'
			GROUP BY observed_at::date
		) p ON d.day = p.day
		LEFT JOIN (
			SELECT created_at::date AS day, COUNT(*) AS cnt
			FROM users
			WHERE created_at >= CURRENT_DATE - $1 * interval '1 day'
			GROUP BY created_at::date
		) u ON d.day = u.day
		ORDER BY d.day`, days)
	if err != nil {
		return nil, fmt.Errorf("activity stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []storage.AdminDayActivity
	for rows.Next() {
		var a storage.AdminDayActivity
		if err := rows.Scan(&a.Date, &a.NewListings, &a.PriceDrops, &a.NewUsers); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (s *Store) DBPoolStats() *storage.DBPoolStats {
	stats := s.db.Stats()
	return &storage.DBPoolStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration.String(),
	}
}

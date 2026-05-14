package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/timeutil"
)

func (s *Store) DBFileSize() (int64, error) {
	if s.dbPath == ":memory:" || strings.HasPrefix(s.dbPath, "file::memory:") {
		return 0, nil
	}

	var total int64
	for _, name := range []string{s.dbPath, s.dbPath + "-wal", s.dbPath + "-shm"} {
		fi, err := os.Stat(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, fmt.Errorf("stat %s: %w", filepath.Base(name), err)
		}
		total += fi.Size()
	}
	return total, nil
}

func (s *Store) CountAllListings(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM listing_history").Scan(&count)
	return count, err
}

func (s *Store) TableSizes(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
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
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM \""+t+"\"").Scan(&count); err != nil {
			return nil, fmt.Errorf("count rows for table %s: %w", t, err)
		}
		sizes[t] = count
	}
	return sizes, nil
}

var purgeable = map[string]bool{
	"listing_history":       true,
	"price_history":         true,
	"seen_listings":         true,
	"listing_user_seen":     true,
	"pending_notifications": true,
	"saved_listings":        true,
	"hidden_listings":       true,
	"pending_digest":        true,
}

func (s *Store) PurgeTable(ctx context.Context, table string) (int64, error) {
	if !purgeable[table] {
		return 0, fmt.Errorf("%w: %q", storage.ErrNotPurgeable, table)
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM \""+table+"\"")
	if err != nil {
		return 0, fmt.Errorf("purge %s: %w", table, err)
	}
	return result.RowsAffected()
}

func (s *Store) AdminListListings(ctx context.Context, limit, offset int, searchID int64) ([]storage.ListingRecord, int64, error) {
	var total int64
	if searchID > 0 {
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM listing_history WHERE search_id = ?", searchID).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count listings: %w", err)
		}
	} else {
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM listing_history").Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count listings: %w", err)
		}
	}

	var rows *sql.Rows
	var err error
	if searchID > 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT token, chat_id, search_id, search_name, manufacturer, model, sub_model, sub_model_id, year, price,
				km, hand, city, page_link, image_url,
				engine_volume, horse_power, engine_type, gear_box, description,
				is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at
			FROM listing_history
			WHERE search_id = ?
			ORDER BY first_seen_at DESC
			LIMIT ? OFFSET ?`, searchID, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT token, chat_id, search_id, search_name, manufacturer, model, sub_model, sub_model_id, year, price,
				km, hand, city, page_link, image_url,
				engine_volume, horse_power, engine_type, gear_box, description,
				is_commercial, fitness_score, median_price, cohort_size, deal_score, base_price, first_seen_at
			FROM listing_history
			ORDER BY first_seen_at DESC
			LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("query listings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []storage.ListingRecord
	for rows.Next() {
		var r storage.ListingRecord
		var score *float64
		var ic sql.NullInt64
		var mp, cs, ds, bp sql.NullInt64
		var firstSeen string
		if err := rows.Scan(
			&r.Token, &r.ChatID, &r.SearchID, &r.SearchName,
			&r.Manufacturer, &r.Model, &r.SubModel, &r.SubModelID, &r.Year, &r.Price,
			&r.Km, &r.Hand, &r.City, &r.PageLink, &r.ImageURL,
			&r.EngineVolume, &r.HorsePower, &r.EngineType, &r.GearBox, &r.Description,
			&ic, &score, &mp, &cs, &ds, &bp, &firstSeen,
		); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}
		r.IsCommercial = storage.ListingCommercialFromSQL(ic)
		r.FitnessScore = score
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
		parsed, parseErr := parseFlexibleTime(firstSeen)
		if parseErr != nil {
			return nil, 0, fmt.Errorf("parse first_seen_at %q for token %s: %w", firstSeen, r.Token, parseErr)
		}
		r.FirstSeenAt = parsed
		items = append(items, r)
	}
	return items, total, rows.Err()
}

func parseFlexibleTime(s string) (time.Time, error) {
	return timeutil.ParseFlexible(s)
}

func (s *Store) AdminDeleteListing(ctx context.Context, token string, chatID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM listing_history WHERE token = ? AND chat_id = ?", token, chatID)
	return err
}

func (s *Store) AdminListSearches(ctx context.Context) ([]storage.Search, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chat_id, user_seq, name, source, manufacturer, model, year_min, year_max,
			price_max, engine_min_cc, max_km, max_hand, keywords, exclude_keys,
			COALESCE(seller_filter, 'any'), active, created_at,
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
	err := s.db.QueryRowContext(ctx, "SELECT chat_id FROM searches WHERE id = ?", id).Scan(&chatID)
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
		"searches", "listing_history", "seen_listings", "listing_user_seen",
		"saved_listings", "hidden_listings",
		"pending_digest",
	} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM \"%s\" WHERE chat_id = ?", table), chatID); err != nil {
			return fmt.Errorf("admin delete user data from %s: %w", table, err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM users WHERE chat_id = ?", chatID); err != nil {
		return fmt.Errorf("admin delete user: %w", err)
	}
	return tx.Commit()
}

func (s *Store) VacuumDB(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("wal checkpoint: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("post-vacuum wal checkpoint: %w", err)
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

func (s *Store) AdminListPriceHistory(ctx context.Context, limit, offset int, token string) ([]storage.AdminPriceRecord, int64, error) {
	var total int64
	var countQ, dataQ string
	var countArgs, dataArgs []any

	if token != "" {
		countQ = "SELECT COUNT(*) FROM price_history WHERE token = ?"
		countArgs = []any{token}
		dataQ = `SELECT ph.token, ph.price, ph.observed_at,
				COALESCE(lh.manufacturer,''), COALESCE(lh.model,''), COALESCE(lh.year,0)
			FROM price_history ph
			LEFT JOIN (
				SELECT token, manufacturer, model, year
				FROM listing_history
				WHERE rowid IN (SELECT MAX(rowid) FROM listing_history GROUP BY token)
			) lh ON ph.token = lh.token
			WHERE ph.token = ?
			ORDER BY ph.observed_at DESC
			LIMIT ? OFFSET ?`
		dataArgs = []any{token, limit, offset}
	} else {
		countQ = "SELECT COUNT(*) FROM price_history"
		dataQ = `SELECT ph.token, ph.price, ph.observed_at,
				COALESCE(lh.manufacturer,''), COALESCE(lh.model,''), COALESCE(lh.year,0)
			FROM price_history ph
			LEFT JOIN (
				SELECT token, manufacturer, model, year
				FROM listing_history
				WHERE rowid IN (SELECT MAX(rowid) FROM listing_history GROUP BY token)
			) lh ON ph.token = lh.token
			ORDER BY ph.observed_at DESC
			LIMIT ? OFFSET ?`
		dataArgs = []any{limit, offset}
	}

	if err := s.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count price history: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query price history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []storage.AdminPriceRecord
	for rows.Next() {
		var r storage.AdminPriceRecord
		var observed string
		if err := rows.Scan(&r.Token, &r.Price, &observed, &r.Manufacturer, &r.Model, &r.Year); err != nil {
			return nil, 0, fmt.Errorf("scan price history: %w", err)
		}
		parsed, parseErr := parseFlexibleTime(observed)
		if parseErr != nil {
			return nil, 0, fmt.Errorf("parse observed_at: %w", parseErr)
		}
		r.ObservedAt = parsed
		items = append(items, r)
	}
	return items, total, rows.Err()
}

func (s *Store) AdminListSeenListings(ctx context.Context, limit, offset int, searchID int64) ([]storage.AdminSeenRecord, int64, error) {
	var total int64
	var countQ, dataQ string
	var countArgs, dataArgs []any

	if searchID > 0 {
		countQ = "SELECT COUNT(*) FROM seen_listings WHERE search_id = ?"
		countArgs = []any{searchID}
		dataQ = `SELECT token, chat_id, search_id, first_seen_at
			FROM seen_listings WHERE search_id = ?
			ORDER BY first_seen_at DESC LIMIT ? OFFSET ?`
		dataArgs = []any{searchID, limit, offset}
	} else {
		countQ = "SELECT COUNT(*) FROM seen_listings"
		dataQ = `SELECT token, chat_id, search_id, first_seen_at
			FROM seen_listings ORDER BY first_seen_at DESC LIMIT ? OFFSET ?`
		dataArgs = []any{limit, offset}
	}

	if err := s.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count seen listings: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query seen listings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []storage.AdminSeenRecord
	for rows.Next() {
		var r storage.AdminSeenRecord
		var firstSeen string
		if err := rows.Scan(&r.Token, &r.ChatID, &r.SearchID, &firstSeen); err != nil {
			return nil, 0, fmt.Errorf("scan seen listing: %w", err)
		}
		parsed, parseErr := parseFlexibleTime(firstSeen)
		if parseErr != nil {
			return nil, 0, fmt.Errorf("parse first_seen_at: %w", parseErr)
		}
		r.FirstSeenAt = parsed
		items = append(items, r)
	}
	return items, total, rows.Err()
}

func (s *Store) AdminActivityStats(ctx context.Context, days int) ([]storage.AdminDayActivity, error) {
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := s.db.QueryContext(ctx, `
		SELECT d.day,
			COALESCE(l.cnt, 0),
			COALESCE(p.cnt, 0),
			COALESCE(u.cnt, 0)
		FROM (
			WITH RECURSIVE dates(day) AS (
				VALUES(?)
				UNION ALL
				SELECT date(day, '+1 day') FROM dates WHERE day < date('now')
			) SELECT day FROM dates
		) d
		LEFT JOIN (
			SELECT date(first_seen_at) AS day, COUNT(*) AS cnt
			FROM listing_history
			WHERE first_seen_at >= ?
			GROUP BY date(first_seen_at)
		) l ON d.day = l.day
		LEFT JOIN (
			SELECT date(observed_at) AS day, COUNT(*) AS cnt
			FROM price_history
			WHERE observed_at >= ?
			GROUP BY date(observed_at)
		) p ON d.day = p.day
		LEFT JOIN (
			SELECT date(created_at) AS day, COUNT(*) AS cnt
			FROM users
			WHERE created_at >= ?
			GROUP BY date(created_at)
		) u ON d.day = u.day
		ORDER BY d.day`, cutoff, cutoff, cutoff, cutoff)
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

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) SetDigestMode(ctx context.Context, chatID int64, mode string, interval string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET digest_mode = ?, digest_interval = ? WHERE chat_id = ?",
		mode, interval, chatID)
	return err
}

func (s *Store) GetDigestMode(ctx context.Context, chatID int64) (string, string, error) {
	var mode, interval string
	err := s.db.QueryRowContext(ctx,
		"SELECT digest_mode, digest_interval FROM users WHERE chat_id = ?",
		chatID).Scan(&mode, &interval)
	if err == sql.ErrNoRows {
		return "instant", "6h", nil
	}
	return mode, interval, err
}

func (s *Store) AddDigestItem(ctx context.Context, chatID int64, payload string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO pending_digest (chat_id, listing_payload) VALUES (?, ?)",
		chatID, payload)
	return err
}

func (s *Store) PeekDigest(ctx context.Context, chatID int64) ([]string, time.Time, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT listing_payload FROM pending_digest WHERE chat_id = ? ORDER BY created_at", chatID)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer func() { _ = rows.Close() }()

	cutoff := time.Now()
	var payloads []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, time.Time{}, err
		}
		payloads = append(payloads, p)
	}
	return payloads, cutoff, rows.Err()
}

func (s *Store) AckDigest(ctx context.Context, chatID int64, before time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM pending_digest WHERE chat_id = ? AND created_at <= ?", chatID, before); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE users SET digest_last_flushed = CURRENT_TIMESTAMP WHERE chat_id = ?", chatID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PendingDigestUsers(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT chat_id FROM pending_digest")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, id)
	}
	return chatIDs, rows.Err()
}

func (s *Store) DigestLastFlushed(ctx context.Context, chatID int64) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx,
		"SELECT digest_last_flushed FROM users WHERE chat_id = ?", chatID).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return t, err
}

func (s *Store) SetDailyDigest(ctx context.Context, chatID int64, enabled bool, digestTime string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET daily_digest = ?, daily_digest_time = ? WHERE chat_id = ?",
		enabled, digestTime, chatID)
	return err
}

func (s *Store) GetDailyDigest(ctx context.Context, chatID int64) (bool, string, time.Time, error) {
	var enabled bool
	var digestTime string
	var lastSent time.Time
	err := s.db.QueryRowContext(ctx,
		"SELECT daily_digest, daily_digest_time, daily_digest_last_sent FROM users WHERE chat_id = ?",
		chatID).Scan(&enabled, &digestTime, &lastSent)
	if err == sql.ErrNoRows {
		return false, "09:00", time.Time{}, nil
	}
	return enabled, digestTime, lastSent, err
}

func (s *Store) UpdateDailyDigestLastSent(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET daily_digest_last_sent = ? WHERE chat_id = ?",
		time.Now(), chatID)
	return err
}

func (s *Store) ListDailyDigestUsers(ctx context.Context) ([]storage.DailyDigestUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, daily_digest_time, daily_digest_last_sent
		FROM users
		WHERE daily_digest = true AND active = true`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []storage.DailyDigestUser
	for rows.Next() {
		var u storage.DailyDigestUser
		if err := rows.Scan(&u.ChatID, &u.DigestTime, &u.LastSent); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) DailyStats(ctx context.Context, chatID int64) ([]storage.DailySearchStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH search_listings AS (
			SELECT s.id AS search_id, s.name AS search_name,
			       lh.price, lh.page_link, lh.first_seen_at
			FROM listing_history lh
			JOIN searches s ON s.name = lh.search_name AND s.chat_id = lh.chat_id
			WHERE lh.chat_id = ? AND s.active = true AND lh.price > 0
		)
		SELECT
			sl.search_name,
			SUM(CASE WHEN sl.first_seen_at >= datetime('now', '-1 day') THEN 1 ELSE 0 END),
			CAST(AVG(sl.price) AS INTEGER),
			MIN(sl.price),
			(SELECT sl2.page_link FROM search_listings sl2
			 WHERE sl2.search_id = sl.search_id
			 ORDER BY sl2.price ASC LIMIT 1),
			AVG(CASE WHEN sl.first_seen_at >= datetime('now', '-1 day') THEN sl.price END),
			AVG(CASE WHEN sl.first_seen_at >= datetime('now', '-7 days')
			          AND sl.first_seen_at < datetime('now', '-1 day') THEN sl.price END)
		FROM search_listings sl
		GROUP BY sl.search_id
		HAVING COUNT(*) >= 5`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stats []storage.DailySearchStats
	for rows.Next() {
		var ds storage.DailySearchStats
		var recentAvg, olderAvg sql.NullFloat64
		if err := rows.Scan(&ds.SearchName, &ds.NewCount, &ds.AvgPrice,
			&ds.BestPrice, &ds.BestPriceLink, &recentAvg, &olderAvg); err != nil {
			return nil, err
		}
		if recentAvg.Valid && olderAvg.Valid && olderAvg.Float64 > 0 {
			ds.PriceTrend = ((recentAvg.Float64 - olderAvg.Float64) / olderAvg.Float64) * 100
		}
		stats = append(stats, ds)
	}
	return stats, rows.Err()
}

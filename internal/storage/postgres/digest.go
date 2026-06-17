package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) SetDigestMode(ctx context.Context, chatID int64, mode string, interval string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET digest_mode = $1, digest_interval = $2 WHERE chat_id = $3`,
		mode, interval, chatID)
	if err != nil {
		return fmt.Errorf("set digest mode: %w", err)
	}
	return nil
}

func (s *Store) GetDigestMode(ctx context.Context, chatID int64) (string, string, error) {
	var mode, interval string
	err := s.db.QueryRowContext(ctx,
		`SELECT digest_mode, digest_interval FROM users WHERE chat_id = $1`,
		chatID).Scan(&mode, &interval)
	if err == sql.ErrNoRows {
		return "digest", "6h", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("get digest mode: %w", err)
	}
	return mode, interval, nil
}

func (s *Store) AddDigestItem(ctx context.Context, chatID int64, payload string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pending_digest (chat_id, listing_payload) VALUES ($1, $2)`,
		chatID, payload)
	if err != nil {
		return fmt.Errorf("add digest item: %w", err)
	}
	return nil
}

func (s *Store) PeekDigest(ctx context.Context, chatID int64) ([]string, time.Time, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT listing_payload FROM pending_digest WHERE chat_id = $1 ORDER BY created_at`,
		chatID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("peek digest: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cutoff := time.Now()
	var payloads []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, time.Time{}, fmt.Errorf("scan digest payload: %w", err)
		}
		payloads = append(payloads, p)
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, fmt.Errorf("peek digest rows: %w", err)
	}
	return payloads, cutoff, nil
}

func (s *Store) AckDigest(ctx context.Context, chatID int64, before time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ack digest begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM pending_digest WHERE chat_id = $1 AND created_at <= $2`,
		chatID, before); err != nil {
		return fmt.Errorf("ack digest delete pending: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET digest_last_flushed = CURRENT_TIMESTAMP WHERE chat_id = $1`,
		chatID); err != nil {
		return fmt.Errorf("ack digest update users: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ack digest commit: %w", err)
	}
	return nil
}

func (s *Store) PendingDigestUsers(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT chat_id FROM pending_digest`)
	if err != nil {
		return nil, fmt.Errorf("pending digest users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var chatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan pending digest chat id: %w", err)
		}
		chatIDs = append(chatIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pending digest users rows: %w", err)
	}
	return chatIDs, nil
}

func (s *Store) DigestLastFlushed(ctx context.Context, chatID int64) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT digest_last_flushed FROM users WHERE chat_id = $1`,
		chatID).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("digest last flushed: %w", err)
	}
	return t, nil
}

func (s *Store) SetDailyDigest(ctx context.Context, chatID int64, enabled bool, digestTime string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET daily_digest = $1, daily_digest_time = $2 WHERE chat_id = $3`,
		enabled, digestTime, chatID)
	if err != nil {
		return fmt.Errorf("set daily digest: %w", err)
	}
	return nil
}

func (s *Store) GetDailyDigest(ctx context.Context, chatID int64) (bool, string, time.Time, error) {
	var enabled bool
	var digestTime string
	var lastSent time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT daily_digest, daily_digest_time, daily_digest_last_sent FROM users WHERE chat_id = $1`,
		chatID).Scan(&enabled, &digestTime, &lastSent)
	if err == sql.ErrNoRows {
		return false, "09:00", time.Time{}, nil
	}
	if err != nil {
		return false, "", time.Time{}, fmt.Errorf("get daily digest: %w", err)
	}
	return enabled, digestTime, lastSent, nil
}

func (s *Store) UpdateDailyDigestLastSent(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET daily_digest_last_sent = $1 WHERE chat_id = $2`,
		time.Now(), chatID)
	if err != nil {
		return fmt.Errorf("update daily digest last sent: %w", err)
	}
	return nil
}

func (s *Store) ListDailyDigestUsers(ctx context.Context) ([]storage.DailyDigestUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, daily_digest_time, daily_digest_last_sent
		FROM users
		WHERE daily_digest = true AND active = true`)
	if err != nil {
		return nil, fmt.Errorf("list daily digest users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []storage.DailyDigestUser
	for rows.Next() {
		var u storage.DailyDigestUser
		if err := rows.Scan(&u.ChatID, &u.DigestTime, &u.LastSent); err != nil {
			return nil, fmt.Errorf("scan daily digest user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list daily digest users rows: %w", err)
	}
	return users, nil
}

func (s *Store) DailyStats(ctx context.Context, chatID int64) ([]storage.DailySearchStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH search_listings AS (
			SELECT s.id AS search_id, s.name AS search_name,
			       lh.price, lh.page_link, lh.first_seen_at
			FROM listing_history lh
			JOIN searches s ON s.id = lh.search_id AND s.chat_id = lh.chat_id
			WHERE lh.chat_id = $1 AND s.active = true AND lh.price > 0
		)
		SELECT
			sl.search_name,
			SUM(CASE WHEN sl.first_seen_at >= NOW() - INTERVAL '1 day' THEN 1 ELSE 0 END),
			CAST(AVG(sl.price) AS BIGINT),
			MIN(sl.price),
			(SELECT sl2.page_link FROM search_listings sl2
			 WHERE sl2.search_id = sl.search_id
			 ORDER BY sl2.price ASC LIMIT 1),
			AVG(CASE WHEN sl.first_seen_at >= NOW() - INTERVAL '1 day' THEN sl.price::double precision END),
			AVG(CASE WHEN sl.first_seen_at >= NOW() - INTERVAL '7 days'
			          AND sl.first_seen_at < NOW() - INTERVAL '1 day' THEN sl.price::double precision END)
		FROM search_listings sl
		GROUP BY sl.search_id, sl.search_name
		HAVING COUNT(*) >= 5`,
		chatID)
	if err != nil {
		return nil, fmt.Errorf("daily stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []storage.DailySearchStats
	for rows.Next() {
		var ds storage.DailySearchStats
		var recentAvg, olderAvg sql.NullFloat64
		if err := rows.Scan(&ds.SearchName, &ds.NewCount, &ds.AvgPrice,
			&ds.BestPrice, &ds.BestPriceLink, &recentAvg, &olderAvg); err != nil {
			return nil, fmt.Errorf("scan daily search stats: %w", err)
		}
		if recentAvg.Valid && olderAvg.Valid && olderAvg.Float64 > 0 {
			ds.PriceTrend = ((recentAvg.Float64 - olderAvg.Float64) / olderAvg.Float64) * 100
		}
		stats = append(stats, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("daily stats rows: %w", err)
	}
	return stats, nil
}

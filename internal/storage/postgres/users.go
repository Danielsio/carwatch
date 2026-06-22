package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	whatsappIDOffset int64 = 1_000_000_000_000
	webIDOffset      int64 = 2_000_000_000_000
)

func (s *Store) UpsertUser(ctx context.Context, chatID int64, username string) error {
	channelID := fmt.Sprintf("%d", chatID)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (chat_id, username, channel_id) VALUES ($1, $2, $3)
		ON CONFLICT (chat_id) DO UPDATE SET
			username = excluded.username,
			channel_id = CASE WHEN users.channel_id = '' THEN excluded.channel_id ELSE users.channel_id END`,
		chatID, username, channelID)
	return err
}

func (s *Store) GetUser(ctx context.Context, chatID int64) (*storage.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id FROM users WHERE chat_id = $1`,
		chatID)

	var u storage.User
	err := row.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
		&u.Tier, &u.TierExpires, &u.TrialUsed, &u.Channel, &u.ChannelID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByChannelID(ctx context.Context, channel, channelID string) (*storage.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id FROM users WHERE channel = $1 AND channel_id = $2`,
		channel, channelID)

	var u storage.User
	err := row.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
		&u.Tier, &u.TierExpires, &u.TrialUsed, &u.Channel, &u.ChannelID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Store) upsertChannelUser(ctx context.Context, channel, channelID, username string, idOffset int64) (int64, error) {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		id, err := s.tryUpsertChannelUser(ctx, channel, channelID, username, idOffset)
		if err == nil {
			return id, nil
		}
		if !isUniqueViolation(err) {
			return 0, err
		}
		slog.Warn("user ID collision, retrying", "channel", channel, "attempt", attempt+1)
	}
	return 0, fmt.Errorf("create %s user: max retries exceeded due to concurrent ID collisions", channel)
}

func (s *Store) tryUpsertChannelUser(ctx context.Context, channel, channelID, username string, idOffset int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("rollback upsert channel user tx", "error", err)
		}
	}()

	var existingID int64
	err = tx.QueryRowContext(ctx,
		`SELECT chat_id FROM users WHERE channel = $1 AND channel_id = $2`,
		channel, channelID).Scan(&existingID)
	if err == nil {
		_ = tx.Commit()
		return existingID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("check existing %s user: %w", channel, err)
	}

	var newID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(chat_id), $1) + 1 FROM users WHERE chat_id >= $2`,
		idOffset-1, idOffset).Scan(&newID); err != nil {
		return 0, fmt.Errorf("next %s user id: %w", channel, err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (chat_id, username, channel, channel_id) VALUES ($1, $2, $3, $4)`,
		newID, username, channel, channelID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newID, nil
}

func (s *Store) UpsertWhatsAppUser(ctx context.Context, phoneNumber string) (int64, error) {
	return s.upsertChannelUser(ctx, "whatsapp", phoneNumber, phoneNumber, whatsappIDOffset)
}

func (s *Store) UpsertWebUser(ctx context.Context, firebaseUID, email string) (int64, error) {
	return s.upsertChannelUser(ctx, "web", firebaseUID, email, webIDOffset)
}

func (s *Store) UpdateUserState(ctx context.Context, chatID int64, state string, stateData string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET state = $1, state_data = $2 WHERE chat_id = $3`,
		state, stateData, chatID)
	return err
}

func (s *Store) ListActiveUsers(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id FROM users WHERE active = true`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsers(rows)
}

func (s *Store) SetUserActive(ctx context.Context, chatID int64, active bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET active = $1 WHERE chat_id = $2`,
		active, chatID)
	return err
}

func (s *Store) ReactivateUsersWithSearches(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET active = true
		WHERE active = false
		  AND chat_id IN (SELECT DISTINCT chat_id FROM searches WHERE active = true)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE active = true`).Scan(&count)
	return count, err
}

func (s *Store) SetUserLanguage(ctx context.Context, chatID int64, lang string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET language = $1 WHERE chat_id = $2`,
		lang, chatID)
	return err
}

func (s *Store) UpdateLastSeenAt(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET last_seen_at = $1 WHERE chat_id = $2`,
		time.Now().UTC(), chatID)
	return err
}

func scanUsers(rows *sql.Rows) ([]storage.User, error) {
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

func (s *Store) SetUserTier(ctx context.Context, chatID int64, tier string, expires time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET tier = $1, tier_expires_at = $2 WHERE chat_id = $3`,
		tier, expires, chatID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) GrantTrial(ctx context.Context, chatID int64, duration time.Duration) error {
	expires := time.Now().Add(duration)
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET tier = 'premium', tier_expires_at = $1, trial_used = true WHERE chat_id = $2 AND trial_used = false`,
		expires, chatID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) LinkTelegramToWeb(ctx context.Context, telegramChatID, webChatID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET linked_web_id = NULL
		WHERE linked_web_id = $1 AND channel = 'telegram' AND chat_id != $2`,
		webChatID, telegramChatID); err != nil {
		return fmt.Errorf("clear previous telegram link: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE users SET linked_web_id = $1
		WHERE chat_id = $2 AND channel = 'telegram'`,
		webChatID, telegramChatID)
	if err != nil {
		return fmt.Errorf("link telegram user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return storage.ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE searches SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM searches s2
			WHERE s2.chat_id = $1
			AND s2.manufacturer = searches.manufacturer
			AND s2.model = searches.model
			AND s2.year_min = searches.year_min
			AND s2.year_max = searches.year_max
			AND s2.price_min = searches.price_min
			AND s2.price_max = searches.price_max
			AND s2.max_km = searches.max_km
			AND s2.max_hand = searches.max_hand
			AND s2.engine_min_cc = searches.engine_min_cc
			AND COALESCE(s2.keywords, '') = COALESCE(searches.keywords, '')
			AND COALESCE(s2.exclude_keys, '') = COALESCE(searches.exclude_keys, '')
			AND COALESCE(s2.seller_filter, '') = COALESCE(searches.seller_filter, '')
			AND COALESCE(s2.gear_box, '') = COALESCE(searches.gear_box, '')
		)`,
		telegramChatID, webChatID); err != nil {
		return fmt.Errorf("migrate searches: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM searches WHERE chat_id = $1`, webChatID); err != nil {
		return fmt.Errorf("cleanup duplicate searches: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE saved_listings SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM saved_listings s2
			WHERE s2.chat_id = $1 AND s2.token = saved_listings.token
		)`,
		telegramChatID, webChatID); err != nil {
		return fmt.Errorf("migrate saved listings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM saved_listings WHERE chat_id = $1`, webChatID); err != nil {
		return fmt.Errorf("cleanup duplicate saved: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE hidden_listings SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM hidden_listings h2
			WHERE h2.chat_id = $1 AND h2.token = hidden_listings.token
		)`,
		telegramChatID, webChatID); err != nil {
		return fmt.Errorf("migrate hidden listings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM hidden_listings WHERE chat_id = $1`, webChatID); err != nil {
		return fmt.Errorf("cleanup duplicate hidden: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE listing_history SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM listing_history lh2
			WHERE lh2.chat_id = $1 AND lh2.token = listing_history.token
		)`,
		telegramChatID, webChatID); err != nil {
		return fmt.Errorf("migrate listing history: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM listing_history WHERE chat_id = $1`, webChatID); err != nil {
		return fmt.Errorf("cleanup duplicate listing history: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE seen_listings SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM seen_listings s2
			WHERE s2.chat_id = $1 AND s2.token = seen_listings.token
		)`,
		telegramChatID, webChatID); err != nil {
		return fmt.Errorf("migrate seen listings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM seen_listings WHERE chat_id = $1`, webChatID); err != nil {
		return fmt.Errorf("cleanup duplicate seen listings: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Store) BackfillLinkedData(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, linked_web_id FROM users
		WHERE linked_web_id IS NOT NULL AND channel = 'telegram' AND active = true`)
	if err != nil {
		return 0, fmt.Errorf("query linked accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type link struct {
		telegramID int64
		webID      int64
	}
	var links []link
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.telegramID, &l.webID); err != nil {
			return 0, fmt.Errorf("scan linked account: %w", err)
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	migrated := 0
	for _, l := range links {
		n, err := s.migrateUserData(ctx, l.telegramID, l.webID)
		if err != nil {
			return migrated, fmt.Errorf("migrate chat_id %d→%d: %w", l.webID, l.telegramID, err)
		}
		migrated += n
	}
	return migrated, nil
}

func (s *Store) migrateUserData(ctx context.Context, telegramChatID, webChatID int64) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var total int64

	res, err := tx.ExecContext(ctx, `
		UPDATE searches SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM searches s2
			WHERE s2.chat_id = $1 AND s2.manufacturer = searches.manufacturer AND s2.model = searches.model
		)`, telegramChatID, webChatID)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err == nil {
		total += n
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM searches WHERE chat_id = $1`, webChatID); err != nil {
		return 0, err
	}

	res, err = tx.ExecContext(ctx, `
		UPDATE saved_listings SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM saved_listings s2
			WHERE s2.chat_id = $1 AND s2.token = saved_listings.token
		)`, telegramChatID, webChatID)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err == nil {
		total += n
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM saved_listings WHERE chat_id = $1`, webChatID); err != nil {
		return 0, err
	}

	res, err = tx.ExecContext(ctx, `
		UPDATE hidden_listings SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM hidden_listings h2
			WHERE h2.chat_id = $1 AND h2.token = hidden_listings.token
		)`, telegramChatID, webChatID)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err == nil {
		total += n
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM hidden_listings WHERE chat_id = $1`, webChatID); err != nil {
		return 0, err
	}

	res, err = tx.ExecContext(ctx, `
		UPDATE listing_history SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM listing_history lh2
			WHERE lh2.chat_id = $1 AND lh2.token = listing_history.token
		)`, telegramChatID, webChatID)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err == nil {
		total += n
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM listing_history WHERE chat_id = $1`, webChatID); err != nil {
		return 0, err
	}

	res, err = tx.ExecContext(ctx, `
		UPDATE seen_listings SET chat_id = $1
		WHERE chat_id = $2
		AND NOT EXISTS (
			SELECT 1 FROM seen_listings s2
			WHERE s2.chat_id = $1 AND s2.token = seen_listings.token
		)`, telegramChatID, webChatID)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err == nil {
		total += n
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM seen_listings WHERE chat_id = $1`, webChatID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return int(total), nil
}

func (s *Store) GetLinkedTelegramUser(ctx context.Context, webChatID int64) (*storage.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id
		FROM users
		WHERE linked_web_id = $1 AND channel = 'telegram' AND active = true
		LIMIT 1`,
		webChatID)

	var u storage.User
	err := row.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
		&u.Tier, &u.TierExpires, &u.TrialUsed, &u.Channel, &u.ChannelID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) ListExpiredPremium(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id
		FROM users
		WHERE tier = 'premium' AND tier_expires_at <= $1 AND tier_expires_at > '1970-01-01 00:00:00+00'::timestamptz`,
		time.Now())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsers(rows)
}

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

const whatsappIDOffset int64 = 1_000_000_000_000

func (s *Store) UpsertUser(ctx context.Context, chatID int64, username string) error {
	channelID := fmt.Sprintf("%d", chatID)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (chat_id, username, channel_id) VALUES (?, ?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET
			username = excluded.username,
			channel_id = CASE WHEN users.channel_id = '' THEN excluded.channel_id ELSE users.channel_id END`,
		chatID, username, channelID)
	return err
}

func (s *Store) GetUser(ctx context.Context, chatID int64) (*storage.User, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id FROM users WHERE chat_id = ?",
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
		"SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used, channel, channel_id FROM users WHERE channel = ? AND channel_id = ?",
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

func (s *Store) UpsertWhatsAppUser(ctx context.Context, phoneNumber string) (int64, error) {
	existing, err := s.GetUserByChannelID(ctx, "whatsapp", phoneNumber)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return existing.ChatID, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var maxID int64
	_ = tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(chat_id), ?) FROM users WHERE chat_id >= ?",
		whatsappIDOffset-1, whatsappIDOffset).Scan(&maxID)
	newID := maxID + 1

	_, err = tx.ExecContext(ctx,
		"INSERT INTO users (chat_id, username, channel, channel_id) VALUES (?, ?, 'whatsapp', ?)",
		newID, phoneNumber, phoneNumber)
	if err != nil {
		return 0, fmt.Errorf("create whatsapp user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return newID, nil
}

func (s *Store) UpdateUserState(ctx context.Context, chatID int64, state string, stateData string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET state = ?, state_data = ? WHERE chat_id = ?",
		state, stateData, chatID)
	return err
}

func (s *Store) ListActiveUsers(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used FROM users WHERE active = true")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsers(rows)
}

func (s *Store) SetUserActive(ctx context.Context, chatID int64, active bool) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET active = ? WHERE chat_id = ?",
		active, chatID)
	return err
}

func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE active = true").Scan(&count)
	return count, err
}

func (s *Store) SetUserLanguage(ctx context.Context, chatID int64, lang string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET language = ? WHERE chat_id = ?",
		lang, chatID)
	return err
}

func scanUsers(rows *sql.Rows) ([]storage.User, error) {
	var users []storage.User
	for rows.Next() {
		var u storage.User
		if err := rows.Scan(&u.ChatID, &u.Username, &u.State, &u.StateData, &u.CreatedAt, &u.Active, &u.Language,
			&u.Tier, &u.TierExpires, &u.TrialUsed); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SetUserTier(ctx context.Context, chatID int64, tier string, expires time.Time) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE users SET tier = ?, tier_expires_at = ? WHERE chat_id = ?",
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
		"UPDATE users SET tier = 'premium', tier_expires_at = ?, trial_used = true WHERE chat_id = ? AND trial_used = false",
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

func (s *Store) ListExpiredPremium(ctx context.Context) ([]storage.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_id, username, state, state_data, created_at, active, language, tier, tier_expires_at, trial_used
		FROM users
		WHERE tier = 'premium' AND tier_expires_at <= ? AND tier_expires_at > '1970-01-01 00:00:00'`,
		time.Now())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsers(rows)
}

package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

const linkTokenTTL = 15 * time.Minute

func generateLinkTokenBytes() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateLinkToken inserts a new link token for web_chat_id with 15-minute expiry.
func (s *Store) CreateLinkToken(ctx context.Context, webChatID int64) (string, error) {
	token, err := generateLinkTokenBytes()
	if err != nil {
		return "", err
	}
	expires := time.Now().UTC().Add(linkTokenTTL)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO link_tokens (token, web_chat_id, expires_at, used)
		VALUES (?, ?, ?, 0)`,
		token, webChatID, expires)
	if err != nil {
		return "", fmt.Errorf("insert link token: %w", err)
	}
	return token, nil
}

// ConsumeLinkToken marks the token used and returns the associated web chat id.
// Returns storage.ErrLinkTokenNotFound, ErrLinkTokenExpired, or ErrLinkTokenUsed when applicable.
func (s *Store) ConsumeLinkToken(ctx context.Context, token string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var webChatID int64
	var used int
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT web_chat_id, used, expires_at FROM link_tokens WHERE token = ?`,
		token).Scan(&webChatID, &used, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, storage.ErrLinkTokenNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("select link token: %w", err)
	}
	if used != 0 {
		return 0, storage.ErrLinkTokenUsed
	}
	if time.Now().UTC().After(expiresAt) {
		return 0, storage.ErrLinkTokenExpired
	}

	res, err := tx.ExecContext(ctx, `UPDATE link_tokens SET used = 1 WHERE token = ? AND used = 0`, token)
	if err != nil {
		return 0, fmt.Errorf("update link token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, storage.ErrLinkTokenUsed
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return webChatID, nil
}

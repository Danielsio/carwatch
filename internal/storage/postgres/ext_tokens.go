package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// CreateExtToken stores a new extension credential for a chat. Only the hash is
// given to us — the plaintext exists exactly once, in the response to the user.
func (s *Store) CreateExtToken(ctx context.Context, chatID int64, tokenHash, label string, expiresAt time.Time) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO ext_tokens (chat_id, token_hash, label, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		chatID, tokenHash, label, expiresAt.UTC()).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create ext token: %w", err)
	}
	return id, nil
}

// extTokenSlidingWindow is how long a token stays valid from its last use, and
// extTokenRenewBefore is how close to expiry a use has to be before the window
// is pushed out again. Renewing on every request would mean a write per
// request; renewing only in the last third of the window keeps the token alive
// indefinitely while it is in use, for one write every few weeks.
const (
	extTokenSlidingWindow = 90 * 24 * time.Hour
	extTokenRenewBefore   = 60 * 24 * time.Hour
	// lastUsedThrottle bounds how often last_used_at is rewritten. The extension
	// calls twice per 15-minute cycle; without a throttle that is a write to the
	// same row ~200 times a day, per user, for a field nobody reads that
	// precisely.
	lastUsedThrottle = time.Hour
)

// ResolveExtToken maps a token hash to its owner, or reports ErrNotFound when
// the token is unknown, revoked, or expired. It doubles as the token's "touch":
// last_used_at and the sliding expiry are refreshed here, both throttled so the
// hot path (every ingest) stays a single indexed statement without a write in
// the common case.
func (s *Store) ResolveExtToken(ctx context.Context, tokenHash string) (int64, error) {
	var chatID int64
	err := s.db.QueryRowContext(ctx, `
		UPDATE ext_tokens SET
			last_used_at = CASE
				WHEN last_used_at IS NULL OR last_used_at < NOW() - $2::interval
				THEN NOW() ELSE last_used_at END,
			expires_at = CASE
				WHEN expires_at < NOW() + $3::interval
				THEN NOW() + $4::interval ELSE expires_at END
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
		RETURNING chat_id`,
		tokenHash,
		lastUsedThrottle.String(),
		extTokenRenewBefore.String(),
		extTokenSlidingWindow.String(),
	).Scan(&chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, storage.ErrNotFound
		}
		return 0, fmt.Errorf("resolve ext token: %w", err)
	}
	return chatID, nil
}

// RevokeExtTokens kills every active token for a chat — the "disconnect my
// extension" button, and the thing to reach for if a token leaks. Returns how
// many were revoked.
func (s *Store) RevokeExtTokens(ctx context.Context, chatID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE ext_tokens SET revoked_at = NOW()
		WHERE chat_id = $1 AND revoked_at IS NULL`, chatID)
	if err != nil {
		return 0, fmt.Errorf("revoke ext tokens: %w", err)
	}
	return res.RowsAffected()
}

// ListExtTokens returns the chat's live tokens (never the secret — it is not
// stored). This is what a "connected browsers" view in Settings reads.
func (s *Store) ListExtTokens(ctx context.Context, chatID int64) ([]storage.ExtToken, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chat_id, label, created_at, last_used_at, expires_at
		FROM ext_tokens
		WHERE chat_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list ext tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []storage.ExtToken
	for rows.Next() {
		var t storage.ExtToken
		var lastUsed sql.NullTime
		if err := rows.Scan(&t.ID, &t.ChatID, &t.Label, &t.CreatedAt, &lastUsed, &t.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan ext token: %w", err)
		}
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.Time
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// PruneExtTokens deletes tokens that have been revoked or expired for longer
// than olderThan. They are already unusable; this just stops the table growing
// forever.
func (s *Store) PruneExtTokens(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).UTC()
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM ext_tokens
		WHERE (revoked_at IS NOT NULL AND revoked_at < $1)
		   OR (expires_at < $1)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune ext tokens: %w", err)
	}
	return res.RowsAffected()
}

// RevokeOldestExtTokens revokes the chat's live tokens beyond the newest `keep`
// of them, so the number of connected browsers stays bounded and a credential
// from a machine the user no longer has cannot linger forever.
func (s *Store) RevokeOldestExtTokens(ctx context.Context, chatID int64, keep int) (int64, error) {
	if keep < 0 {
		keep = 0
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE ext_tokens SET revoked_at = NOW()
		WHERE id IN (
			SELECT id FROM ext_tokens
			WHERE chat_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
			ORDER BY created_at DESC
			OFFSET $2
		)`, chatID, keep)
	if err != nil {
		return 0, fmt.Errorf("revoke oldest ext tokens: %w", err)
	}
	return res.RowsAffected()
}

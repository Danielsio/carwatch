package postgres

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) ClaimNew(ctx context.Context, token string, chatID int64, searchID int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO seen_listings (token, chat_id, search_id) VALUES ($1, $2, $3)
		ON CONFLICT (token, chat_id) DO NOTHING`,
		token, chatID, searchID)
	if err != nil {
		return false, fmt.Errorf("claim new: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim new rows affected: %w", err)
	}
	return rows > 0, nil
}

func (s *Store) ReleaseClaim(ctx context.Context, token string, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM seen_listings WHERE token = $1 AND chat_id = $2`,
		token, chatID)
	if err != nil {
		return fmt.Errorf("release claim: %w", err)
	}
	return nil
}

func (s *Store) Prune(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM seen_listings WHERE first_seen_at < $1`,
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune seen listings: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune seen listings rows affected: %w", err)
	}
	return rows, nil
}

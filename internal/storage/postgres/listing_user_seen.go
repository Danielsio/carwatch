package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) MarkListingUserSeen(ctx context.Context, chatID int64, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO listing_user_seen (chat_id, token, seen_at) VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, token) DO UPDATE SET seen_at = EXCLUDED.seen_at`,
		chatID, token, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("mark listing user seen: %w", err)
	}
	return nil
}

func (s *Store) UnmarkListingUserSeen(ctx context.Context, chatID int64, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM listing_user_seen WHERE chat_id = $1 AND token = $2`, chatID, token)
	if err != nil {
		return fmt.Errorf("unmark listing user seen: %w", err)
	}
	return nil
}

func (s *Store) ListingUserSeenAmong(ctx context.Context, chatID int64, tokens []string) (map[string]bool, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	uniq := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		uniq = append(uniq, t)
	}
	if len(uniq) == 0 {
		return nil, nil
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin read tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	out := make(map[string]bool)
	for start := 0; start < len(uniq); start += savedAmongBatchSize {
		end := start + savedAmongBatchSize
		if end > len(uniq) {
			end = len(uniq)
		}
		batch := uniq[start:end]

		placeholders := make([]string, len(batch))
		args := make([]any, 0, 1+len(batch))
		args = append(args, chatID)
		for i, t := range batch {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, t)
		}
		q := `SELECT token FROM listing_user_seen WHERE chat_id = $1 AND token IN (` + strings.Join(placeholders, ", ") + `)`
		rows, err := tx.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var tok string
			if err := rows.Scan(&tok); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[tok] = true
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return out, tx.Commit()
}

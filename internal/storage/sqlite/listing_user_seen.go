package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MarkListingUserSeen records that the user dismissed this token from their "new" feed.
func (s *Store) MarkListingUserSeen(ctx context.Context, chatID int64, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO listing_user_seen (chat_id, token, seen_at) VALUES (?, ?, ?)
		ON CONFLICT(chat_id, token) DO UPDATE SET seen_at = excluded.seen_at`,
		chatID, token, time.Now().UTC().Format(time.RFC3339Nano))
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
		`DELETE FROM listing_user_seen WHERE chat_id = ? AND token = ?`, chatID, token)
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

		args := make([]interface{}, 0, 1+len(batch))
		args = append(args, chatID)
		for _, t := range batch {
			args = append(args, t)
		}
		placeholders := ""
		for i := range batch {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
		}

		rows, err := tx.QueryContext(ctx,
			"SELECT token FROM listing_user_seen WHERE chat_id = ? AND token IN ("+placeholders+")",
			args...)
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

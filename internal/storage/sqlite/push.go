package sqlite

import (
	"context"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) SavePushSubscription(ctx context.Context, sub storage.PushSubscription) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (chat_id, endpoint, p256dh, auth)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
			chat_id = excluded.chat_id,
			p256dh  = excluded.p256dh,
			auth    = excluded.auth`,
		sub.ChatID, sub.Endpoint, sub.P256DH, sub.Auth)
	return err
}

func (s *Store) DeletePushSubscription(ctx context.Context, chatID int64, endpoint string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE chat_id = ? AND endpoint = ?`,
		chatID, endpoint)
	return err
}

func (s *Store) ListPushSubscriptions(ctx context.Context, chatID int64) ([]storage.PushSubscription, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, chat_id, endpoint, p256dh, auth, created_at
		 FROM push_subscriptions WHERE chat_id = ? ORDER BY created_at DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var subs []storage.PushSubscription
	for rows.Next() {
		var sub storage.PushSubscription
		if err := rows.Scan(&sub.ID, &sub.ChatID, &sub.Endpoint, &sub.P256DH, &sub.Auth, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

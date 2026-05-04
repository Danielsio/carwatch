package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) EnqueueNotification(ctx context.Context, recipient, searchName, payload string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pending_notifications (recipient, search_name, payload) VALUES ($1, $2, $3)`,
		recipient, searchName, payload)
	if err != nil {
		return fmt.Errorf("enqueue notification: %w", err)
	}
	return nil
}

const pendingNotificationsBatchSize = 500

func (s *Store) PendingNotifications(ctx context.Context) ([]storage.PendingNotification, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, recipient, search_name, payload FROM pending_notifications ORDER BY created_at LIMIT $1`,
		pendingNotificationsBatchSize)
	if err != nil {
		return nil, fmt.Errorf("pending notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pending []storage.PendingNotification
	for rows.Next() {
		var p storage.PendingNotification
		if err := rows.Scan(&p.ID, &p.Recipient, &p.SearchName, &p.Payload); err != nil {
			return nil, fmt.Errorf("scan pending notification: %w", err)
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pending notifications rows: %w", err)
	}
	return pending, nil
}

func (s *Store) AckNotification(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM pending_notifications WHERE id = $1`,
		id)
	if err != nil {
		return fmt.Errorf("ack notification: %w", err)
	}
	return nil
}

func (s *Store) PruneNotifications(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM pending_notifications WHERE created_at < $1`,
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune notifications: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune notifications rows affected: %w", err)
	}
	return n, nil
}

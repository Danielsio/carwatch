package sqlite

import (
	"context"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func (s *Store) EnqueueNotification(ctx context.Context, recipient, searchName, payload string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO pending_notifications (recipient, search_name, payload) VALUES (?, ?, ?)",
		recipient, searchName, payload)
	return err
}

func (s *Store) PendingNotifications(ctx context.Context) ([]storage.PendingNotification, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, recipient, search_name, payload FROM pending_notifications ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var pending []storage.PendingNotification
	for rows.Next() {
		var p storage.PendingNotification
		if err := rows.Scan(&p.ID, &p.Recipient, &p.SearchName, &p.Payload); err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

func (s *Store) AckNotification(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM pending_notifications WHERE id = ?", id)
	return err
}

func (s *Store) PruneNotifications(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, "DELETE FROM pending_notifications WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

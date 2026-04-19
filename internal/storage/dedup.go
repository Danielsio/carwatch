package storage

import (
	"context"
	"time"
)

type DedupStore interface {
	ClaimNew(ctx context.Context, token string, searchName string) (bool, error)
	ReleaseClaim(ctx context.Context, token string) error
	Prune(ctx context.Context, olderThan time.Duration) (int64, error)
	Close() error
}

type PendingNotification struct {
	ID         int64
	Recipient  string
	SearchName string
	Payload    string
}

type NotificationQueue interface {
	EnqueueNotification(ctx context.Context, recipient, searchName, payload string) error
	PendingNotifications(ctx context.Context) ([]PendingNotification, error)
	AckNotification(ctx context.Context, id int64) error
}

type PriceTracker interface {
	RecordPrice(ctx context.Context, token string, price int) (oldPrice int, changed bool, err error)
}

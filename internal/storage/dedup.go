package storage

import (
	"context"
	"time"
)

type DedupStore interface {
	HasSeen(ctx context.Context, token string) (bool, error)
	MarkSeen(ctx context.Context, token string, searchName string) error
	Prune(ctx context.Context, olderThan time.Duration) (int64, error)
	Close() error
}

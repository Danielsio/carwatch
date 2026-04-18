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

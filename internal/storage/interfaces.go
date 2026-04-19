package storage

import (
	"context"
	"time"
)

type User struct {
	ChatID    int64
	Username  string
	State     string
	StateData string
	CreatedAt time.Time
	Active    bool
}

type Search struct {
	ID           int64
	ChatID       int64
	Name         string
	Source       string
	Manufacturer int
	Model        int
	YearMin      int
	YearMax      int
	PriceMax     int
	EngineMinCC  int
	MaxKm        int
	MaxHand      int
	Active       bool
	CreatedAt    time.Time
}

type UserStore interface {
	UpsertUser(ctx context.Context, chatID int64, username string) error
	GetUser(ctx context.Context, chatID int64) (*User, error)
	UpdateUserState(ctx context.Context, chatID int64, state string, stateData string) error
	ListActiveUsers(ctx context.Context) ([]User, error)
	SetUserActive(ctx context.Context, chatID int64, active bool) error
	CountUsers(ctx context.Context) (int64, error)
}

type SearchStore interface {
	CreateSearch(ctx context.Context, s Search) (int64, error)
	ListSearches(ctx context.Context, chatID int64) ([]Search, error)
	GetSearch(ctx context.Context, id int64) (*Search, error)
	DeleteSearch(ctx context.Context, id int64, chatID int64) error
	SetSearchActive(ctx context.Context, id int64, active bool) error
	ListAllActiveSearches(ctx context.Context) ([]Search, error)
	CountSearches(ctx context.Context, chatID int64) (int64, error)
	CountAllSearches(ctx context.Context) (int64, error)
}

type DedupStore interface {
	ClaimNew(ctx context.Context, token string, chatID int64, searchID int64) (bool, error)
	ReleaseClaim(ctx context.Context, token string, chatID int64) error
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

type DigestStore interface {
	SetDigestMode(ctx context.Context, chatID int64, mode string, interval string) error
	GetDigestMode(ctx context.Context, chatID int64) (mode string, interval string, err error)
	AddDigestItem(ctx context.Context, chatID int64, payload string) error
	FlushDigest(ctx context.Context, chatID int64) ([]string, error)
	PendingDigestUsers(ctx context.Context) ([]int64, error)
	DigestLastFlushed(ctx context.Context, chatID int64) (time.Time, error)
}

type ListingRecord struct {
	Token        string
	SearchName   string
	Manufacturer string
	Model        string
	Year         int
	Price        int
	Km           int
	Hand         int
	City         string
	PageLink     string
	FirstSeenAt  time.Time
}

type ListingStore interface {
	SaveListing(ctx context.Context, r ListingRecord) error
}

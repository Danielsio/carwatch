package storage

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ChatID       int64
	Username     string
	State        string
	StateData    string
	CreatedAt    time.Time
	Active       bool
	Language     string
	Tier         string
	TierExpires  time.Time
	TrialUsed    bool
	Channel      string
	ChannelID    string
}

type Search struct {
	ID           int64
	ChatID       int64
	UserSeq      int
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
	Keywords     string
	ExcludeKeys  string
	Active       bool
	CreatedAt    time.Time
	ShareToken   string
}

type UserStore interface {
	UpsertUser(ctx context.Context, chatID int64, username string) error
	GetUser(ctx context.Context, chatID int64) (*User, error)
	GetUserByChannelID(ctx context.Context, channel, channelID string) (*User, error)
	UpsertWhatsAppUser(ctx context.Context, phoneNumber string) (int64, error)
	UpsertWebUser(ctx context.Context, firebaseUID, email string) (int64, error)
	UpdateUserState(ctx context.Context, chatID int64, state string, stateData string) error
	ListActiveUsers(ctx context.Context) ([]User, error)
	SetUserActive(ctx context.Context, chatID int64, active bool) error
	SetUserLanguage(ctx context.Context, chatID int64, lang string) error
	UpdateLastSeenAt(ctx context.Context, chatID int64) error
	CountUsers(ctx context.Context) (int64, error)
	SetUserTier(ctx context.Context, chatID int64, tier string, expires time.Time) error
	GrantTrial(ctx context.Context, chatID int64, duration time.Duration) error
	ListExpiredPremium(ctx context.Context) ([]User, error)
}

type SearchStore interface {
	CreateSearch(ctx context.Context, s Search) (int64, error)
	UpdateSearch(ctx context.Context, s Search) error
	ListSearches(ctx context.Context, chatID int64) ([]Search, error)
	GetSearch(ctx context.Context, id int64) (*Search, error)
	GetSearchBySeq(ctx context.Context, chatID int64, seq int) (*Search, error)
	GetSearchByShareToken(ctx context.Context, token string) (*Search, error)
	DeleteSearch(ctx context.Context, id int64, chatID int64) error
	SetSearchActive(ctx context.Context, id int64, chatID int64, active bool) error
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
	PruneNotifications(ctx context.Context, olderThan time.Duration) (int64, error)
}

type PriceTracker interface {
	RecordPrice(ctx context.Context, token string, price int) (oldPrice int, changed bool, err error)
	PrunePrices(ctx context.Context, olderThan time.Duration) (int64, error)
	GetPriceHistory(ctx context.Context, token string) ([]PricePoint, error)
}

type DigestStore interface {
	SetDigestMode(ctx context.Context, chatID int64, mode string, interval string) error
	GetDigestMode(ctx context.Context, chatID int64) (mode string, interval string, err error)
	AddDigestItem(ctx context.Context, chatID int64, payload string) error
	PeekDigest(ctx context.Context, chatID int64) ([]string, time.Time, error)
	AckDigest(ctx context.Context, chatID int64, before time.Time) error
	PendingDigestUsers(ctx context.Context) ([]int64, error)
	DigestLastFlushed(ctx context.Context, chatID int64) (time.Time, error)
}

type ListingRecord struct {
	Token        string
	ChatID       int64
	SearchID     int64
	SearchName   string
	Manufacturer string
	Model        string
	Year         int
	Price        int
	Km           int
	Hand         int
	City         string
	PageLink     string
	ImageURL     string
	FitnessScore *float64
	FirstSeenAt  time.Time
}

type PricePoint struct {
	Price      int
	ObservedAt time.Time
}

type ListingStore interface {
	SaveListing(ctx context.Context, r ListingRecord) error
	SaveListings(ctx context.Context, records []ListingRecord) error
	GetListing(ctx context.Context, chatID int64, token string) (*ListingRecord, error)
	ListUserListings(ctx context.Context, chatID int64, limit, offset int) ([]ListingRecord, error)
	CountUserListings(ctx context.Context, chatID int64) (int64, error)
	ListSearchListings(ctx context.Context, chatID int64, searchID int64, limit, offset int, sort string) ([]ListingRecord, error)
	CountSearchListings(ctx context.Context, chatID int64, searchID int64) (int64, error)
}

type SavedListingStore interface {
	SaveBookmark(ctx context.Context, chatID int64, token string) error
	RemoveBookmark(ctx context.Context, chatID int64, token string) error
	ListSaved(ctx context.Context, chatID int64, limit, offset int) ([]ListingRecord, error)
	CountSaved(ctx context.Context, chatID int64) (int64, error)
	// IsSaved reports whether the token is bookmarked for chatID.
	IsSaved(ctx context.Context, chatID int64, token string) (bool, error)
	// SavedAmong returns, for each token in tokens that is bookmarked for chatID, an entry set to true.
	SavedAmong(ctx context.Context, chatID int64, tokens []string) (map[string]bool, error)
}

type HiddenListingStore interface {
	HideListing(ctx context.Context, chatID int64, token string) error
	UnhideListing(ctx context.Context, chatID int64, token string) error
	IsHidden(ctx context.Context, chatID int64, token string) (bool, error)
	ListHiddenTokens(ctx context.Context, chatID int64) (map[string]bool, error)
	ListHidden(ctx context.Context, chatID int64, limit, offset int) ([]string, error)
	CountHidden(ctx context.Context, chatID int64) (int64, error)
	ClearHidden(ctx context.Context, chatID int64) error
}

type MarketListing struct {
	Manufacturer string
	Model        string
	Year         int
	Price        int
}

type MarketStore interface {
	MarketListings(ctx context.Context) ([]MarketListing, error)
}

type DailyDigestUser struct {
	ChatID     int64
	DigestTime string
	LastSent   time.Time
}

type DailySearchStats struct {
	SearchName    string
	NewCount      int
	AvgPrice      int
	BestPrice     int
	BestPriceLink string
	PriceTrend    float64
}

type DailyDigestStore interface {
	SetDailyDigest(ctx context.Context, chatID int64, enabled bool, digestTime string) error
	GetDailyDigest(ctx context.Context, chatID int64) (enabled bool, digestTime string, lastSent time.Time, err error)
	UpdateDailyDigestLastSent(ctx context.Context, chatID int64) error
	ListDailyDigestUsers(ctx context.Context) ([]DailyDigestUser, error)
	DailyStats(ctx context.Context, chatID int64) ([]DailySearchStats, error)
}

type AdminStore interface {
	DBFileSize() (int64, error)
	CountAllListings(ctx context.Context) (int64, error)
	TableSizes(ctx context.Context) (map[string]int64, error)
	PurgeTable(ctx context.Context, table string) (int64, error)
	AdminListListings(ctx context.Context, limit, offset int) ([]ListingRecord, int64, error)
	AdminDeleteListing(ctx context.Context, token string, chatID int64) error
	VacuumDB(ctx context.Context) error
}

type NotificationStore interface {
	NewListingsSince(ctx context.Context, chatID int64, since time.Time, limit, offset int) ([]ListingRecord, error)
	CountNewListingsSince(ctx context.Context, chatID int64, since time.Time) (int64, error)
	GetLastSeenAt(ctx context.Context, chatID int64) (time.Time, error)
}

type CatalogEntry struct {
	ManufacturerID   int
	ManufacturerName string
	ModelID          int
	ModelName        string
}

type CatalogStore interface {
	SaveCatalogEntries(ctx context.Context, entries []CatalogEntry) error
	LoadCatalogEntries(ctx context.Context) ([]CatalogEntry, error)
	CatalogAge(ctx context.Context) (time.Duration, error)
}

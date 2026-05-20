package storage

import "database/sql"

// Store is the combined interface that postgres.Store satisfies.
type Store interface {
	UserStore
	LinkTokenStore
	SearchStore
	DedupStore
	NotificationQueue
	PriceTracker
	DigestStore
	ListingStore
	SavedListingStore
	HiddenListingStore
	MarketStore
	PriceListStore
	DailyDigestStore
	AdminStore
	NotificationStore
	PushSubscriptionStore

	DB() *sql.DB
	DBSizeBytes() (int64, error)
}

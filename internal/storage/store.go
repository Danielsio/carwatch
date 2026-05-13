package storage

import "database/sql"

// Store is the combined interface that both sqlite.Store and postgres.Store satisfy.
// It embeds all individual store interfaces plus lifecycle methods.
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

	DB() *sql.DB
	DBSizeBytes() (int64, error)
}

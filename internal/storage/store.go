package storage

import "database/sql"

// DBAccessor provides raw database access for admin operations and test helpers.
type DBAccessor interface {
	DB() *sql.DB
	DBSizeBytes() (int64, error)
}

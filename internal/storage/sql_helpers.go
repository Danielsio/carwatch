package storage

import "database/sql"

// ListingCommercialToSQL converts a tri-state commercial flag for SQL INTEGER NULL/0/1.
func ListingCommercialToSQL(c *bool) any {
	if c == nil {
		return nil
	}
	if *c {
		return 1
	}
	return 0
}

// ListingCommercialFromSQL scans a nullable INTEGER into *bool (nil if SQL NULL).
func ListingCommercialFromSQL(nt sql.NullInt64) *bool {
	if !nt.Valid {
		return nil
	}
	v := nt.Int64 != 0
	return &v
}

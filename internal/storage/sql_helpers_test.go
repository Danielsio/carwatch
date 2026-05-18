package storage

import (
	"database/sql"
	"testing"
)

func TestListingCommercialToSQL(t *testing.T) {
	if got := ListingCommercialToSQL(nil); got != nil {
		t.Errorf("nil -> expected nil, got %v", got)
	}

	tr := true
	if got := ListingCommercialToSQL(&tr); got != 1 {
		t.Errorf("true -> expected 1, got %v", got)
	}

	fl := false
	if got := ListingCommercialToSQL(&fl); got != 0 {
		t.Errorf("false -> expected 0, got %v", got)
	}
}

func TestListingCommercialFromSQL(t *testing.T) {
	if got := ListingCommercialFromSQL(sql.NullInt64{Valid: false}); got != nil {
		t.Errorf("NULL -> expected nil, got %v", got)
	}

	got := ListingCommercialFromSQL(sql.NullInt64{Int64: 0, Valid: true})
	if got == nil || *got != false {
		t.Errorf("0 -> expected false, got %v", got)
	}

	got = ListingCommercialFromSQL(sql.NullInt64{Int64: 1, Valid: true})
	if got == nil || *got != true {
		t.Errorf("1 -> expected true, got %v", got)
	}

	got = ListingCommercialFromSQL(sql.NullInt64{Int64: 42, Valid: true})
	if got == nil || *got != true {
		t.Errorf("42 -> expected true (non-zero), got %v", got)
	}
}

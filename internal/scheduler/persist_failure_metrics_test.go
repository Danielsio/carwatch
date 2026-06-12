package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

// TestPersistListings_RecordsFailureMetrics verifies that a failed batch
// persist increments the persist-failure counter, and that failed dedup-claim
// releases (the permanent listing-loss path, F10) increment the claim-release
// counter — once per record.
func TestPersistListings_RecordsFailureMetrics(t *testing.T) {
	obs := &countingObserver{}
	dedup := newErrDedup()
	dedup.releaseErr = errors.New("release failed")

	s := &Scheduler{
		logger:   testLogger(),
		observer: obs,
		stores: Stores{
			Dedup:    dedup,
			Listings: &errListingStore{saveErr: errors.New("db write failed")},
		},
	}

	records := []storage.ListingRecord{
		{Token: "tok-a", ChatID: 100, SearchID: 1},
		{Token: "tok-b", ChatID: 100, SearchID: 1},
	}
	if err := s.persistListings(context.Background(), records, testLogger()); err == nil {
		t.Fatal("expected persist error")
	}

	if got := obs.persistFailures.Load(); got != 1 {
		t.Errorf("persistFailures = %d, want 1", got)
	}
	if got := obs.claimReleaseFailures.Load(); got != 2 {
		t.Errorf("claimReleaseFailures = %d, want 2 (one per record)", got)
	}
}

// TestPersistListings_NoFailureMetricsOnSuccess verifies the happy path keeps
// both counters at zero.
func TestPersistListings_NoFailureMetricsOnSuccess(t *testing.T) {
	obs := &countingObserver{}
	s := &Scheduler{
		logger:   testLogger(),
		observer: obs,
		stores: Stores{
			Dedup:    newMockDedup(),
			Listings: &mockListingStore{},
		},
	}

	records := []storage.ListingRecord{{Token: "tok-a", ChatID: 100, SearchID: 1}}
	if err := s.persistListings(context.Background(), records, testLogger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := obs.persistFailures.Load(); got != 0 {
		t.Errorf("persistFailures = %d, want 0", got)
	}
	if got := obs.claimReleaseFailures.Load(); got != 0 {
		t.Errorf("claimReleaseFailures = %d, want 0", got)
	}
}

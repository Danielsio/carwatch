package postgres

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// TestPostgres_Dedup_ConcurrentClaim verifies the atomicity guarantee that the
// entire notification pipeline depends on: when many goroutines race to claim
// the same (token, chat_id, search_id), exactly one wins. This is the property
// that prevents duplicate notification delivery, and it was previously only
// covered by sequential tests.
func TestPostgres_Dedup_ConcurrentClaim(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	const goroutines = 32
	var (
		wg      sync.WaitGroup
		winners atomic.Int64
		errs    atomic.Int64
		start   = make(chan struct{})
	)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start // release all goroutines at once to maximize contention
			isNew, err := store.ClaimNew(ctx, "race-token", 100, 1)
			if err != nil {
				errs.Add(1)
				return
			}
			if isNew {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := errs.Load(); got != 0 {
		t.Fatalf("expected no claim errors, got %d", got)
	}
	if got := winners.Load(); got != 1 {
		t.Fatalf("expected exactly one winning claim, got %d", got)
	}
}

// TestPostgres_Dedup_ConcurrentReleaseReclaim verifies that a released claim can
// be re-won by exactly one of several racing claimers, with no lost or double
// grants.
func TestPostgres_Dedup_ConcurrentReleaseReclaim(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	// Seed the claim, then release it so the slot is free for the race.
	if isNew, err := store.ClaimNew(ctx, "reclaim-token", 100, 1); err != nil || !isNew {
		t.Fatalf("seed claim: new=%v err=%v", isNew, err)
	}
	if err := store.ReleaseClaim(ctx, "reclaim-token", 100, 1); err != nil {
		t.Fatalf("release: %v", err)
	}

	const goroutines = 16
	var (
		wg      sync.WaitGroup
		winners atomic.Int64
		start   = make(chan struct{})
	)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if isNew, err := store.ClaimNew(ctx, "reclaim-token", 100, 1); err == nil && isNew {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := winners.Load(); got != 1 {
		t.Fatalf("expected exactly one re-claim winner, got %d", got)
	}
}

// TestPostgres_Dedup_ConcurrentDistinctSearches verifies that claims for the
// same token under different searches are independent: concurrent claims across
// distinct search IDs each succeed exactly once and never block each other.
func TestPostgres_Dedup_ConcurrentDistinctSearches(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	const searches = 24
	var (
		wg      sync.WaitGroup
		winners atomic.Int64
		errs    atomic.Int64
		start   = make(chan struct{})
	)
	wg.Add(searches)
	for s := 0; s < searches; s++ {
		searchID := int64(s + 1)
		go func() {
			defer wg.Done()
			<-start
			isNew, err := store.ClaimNew(ctx, "shared-token", 100, searchID)
			if err != nil {
				errs.Add(1)
				return
			}
			if isNew {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := errs.Load(); got != 0 {
		t.Fatalf("expected no claim errors, got %d", got)
	}
	if got := winners.Load(); got != int64(searches) {
		t.Fatalf("expected every distinct-search claim to win, got %d of %d", got, searches)
	}
}

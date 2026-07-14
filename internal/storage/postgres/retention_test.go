package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// The bug this guards: cars sit on Yad2 for months and the extension re-pushes
// every still-matching listing every 15 minutes, but a dedup claim was pruned
// on a fixed timer from FIRST sight. Once the retention window elapsed under a
// live listing, the next push re-claimed it as new and the user was re-alerted
// about a car they had already been shown — again every 30 days, for as long as
// the ad stayed up.
func TestDedupPrune_KeepsClaimsForListingsStillBeingSeen(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	const (
		token    = "still-for-sale"
		chatID   = int64(1)
		searchID = int64(1)
	)

	isNew, err := store.ClaimNew(ctx, token, chatID, searchID)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !isNew {
		t.Fatal("first sighting of a listing must be new (the user gets one alert)")
	}

	// Age the claim past any plausible retention window, as if the car had been
	// listed for months.
	if _, err := store.db.ExecContext(ctx,
		`UPDATE seen_listings SET first_seen_at = NOW() - INTERVAL '90 days',
		                          last_seen_at  = NOW() - INTERVAL '90 days'
		 WHERE token = $1`, token); err != nil {
		t.Fatalf("age claim: %v", err)
	}

	// The extension pushes it again, because it is still on Yad2. This must not
	// alert (it is not new) and must renew the claim's retention clock.
	isNew, err = store.ClaimNew(ctx, token, chatID, searchID)
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if isNew {
		t.Fatal("a listing we have already claimed must never report as new again")
	}

	// The daily prune now runs with a 30-day window.
	pruned, err := store.Prune(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("prune deleted %d claim(s) for a listing that is still being seen", pruned)
	}

	// The real assertion: the next push after the prune must still be a
	// duplicate, not a fresh alert.
	isNew, err = store.ClaimNew(ctx, token, chatID, searchID)
	if err != nil {
		t.Fatalf("claim after prune: %v", err)
	}
	if isNew {
		t.Fatal("listing re-alerted after the prune — the duplicate-alert bug is back")
	}
}

// Retention still works: a listing that really is gone from the source stops
// being re-claimed, ages out, and is pruned.
func TestDedupPrune_DropsClaimsForListingsGoneFromTheSource(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	const (
		token    = "sold-long-ago"
		chatID   = int64(1)
		searchID = int64(1)
	)

	if _, err := store.ClaimNew(ctx, token, chatID, searchID); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// Nobody has re-observed it for 40 days: the car is gone.
	if _, err := store.db.ExecContext(ctx,
		`UPDATE seen_listings SET last_seen_at = NOW() - INTERVAL '40 days' WHERE token = $1`,
		token); err != nil {
		t.Fatalf("age claim: %v", err)
	}

	pruned, err := store.Prune(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected the stale claim to be pruned, got %d", pruned)
	}
}

// Same shape for listing_history: a live listing must not be deleted and
// re-inserted (which reset first_seen_at, resurfacing it in the UI as newly
// found and clearing its enrichment counters).
func TestPruneListings_KeepsListingsStillBeingSeen(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	const chatID = int64(1)
	rec := storage.ListingRecord{
		Token:       "long-lived-ad",
		ChatID:      chatID,
		SearchID:    1,
		Price:       50000,
		Year:        2019,
		FirstSeenAt: time.Now().Add(-120 * 24 * time.Hour),
	}
	if err := store.SaveListing(ctx, rec); err != nil {
		t.Fatalf("save listing: %v", err)
	}

	// It was first seen 120 days ago but the source still serves it, so the
	// most recent push re-upserted it just now (SaveListing refreshes
	// last_seen_at) — except on the very first insert, where the default is NOW.
	if _, err := store.db.ExecContext(ctx,
		`UPDATE listing_history SET first_seen_at = NOW() - INTERVAL '120 days' WHERE token = $1`,
		rec.Token); err != nil {
		t.Fatalf("age listing: %v", err)
	}
	// Re-observe it, as the extension would.
	if err := store.SaveListing(ctx, rec); err != nil {
		t.Fatalf("re-save listing: %v", err)
	}

	pruned, err := store.PruneListings(ctx, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("prune listings: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("prune deleted %d live listing(s) that the source is still serving", pruned)
	}

	got, err := store.GetListing(ctx, chatID, rec.Token)
	if err != nil {
		t.Fatalf("get listing: %v", err)
	}
	if got == nil {
		t.Fatal("live listing was pruned")
	}
}

func TestPruneListings_DropsListingsGoneFromTheSource(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	rec := storage.ListingRecord{
		Token:       "vanished-ad",
		ChatID:      1,
		SearchID:    1,
		Price:       50000,
		FirstSeenAt: time.Now().Add(-120 * 24 * time.Hour),
	}
	if err := store.SaveListing(ctx, rec); err != nil {
		t.Fatalf("save listing: %v", err)
	}
	if _, err := store.db.ExecContext(ctx,
		`UPDATE listing_history SET last_seen_at = NOW() - INTERVAL '100 days' WHERE token = $1`,
		rec.Token); err != nil {
		t.Fatalf("age listing: %v", err)
	}

	pruned, err := store.PruneListings(ctx, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("prune listings: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected the long-gone listing to be pruned, got %d", pruned)
	}
}

// A price of 0 is what the parser yields when the feed omits the field — not a
// car that became free. Letting it overwrite a known price erased the very
// signal the product exists to track.
func TestSaveListing_ZeroPriceDoesNotEraseAKnownPrice(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	const chatID = int64(1)
	rec := storage.ListingRecord{
		Token:       "priced-car",
		ChatID:      chatID,
		SearchID:    1,
		Price:       75000,
		Hand:        2,
		Km:          40000,
		FirstSeenAt: time.Now(),
	}
	if err := store.SaveListing(ctx, rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	// A partial payload arrives: price and hand missing (parsed as 0).
	partial := rec
	partial.Price = 0
	partial.Hand = 0
	partial.Km = 0
	if err := store.SaveListing(ctx, partial); err != nil {
		t.Fatalf("save partial: %v", err)
	}

	got, err := store.GetListing(ctx, chatID, rec.Token)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Price != 75000 {
		t.Errorf("price was erased by a partial payload: got %d, want 75000", got.Price)
	}
	if got.Hand != 2 {
		t.Errorf("hand was erased by a partial payload: got %d, want 2", got.Hand)
	}
	if got.Km != 40000 {
		t.Errorf("km was erased by a partial payload: got %d, want 40000", got.Km)
	}
}

// The guard must not freeze prices: a real price change is the product's core
// signal and still has to land.
func TestSaveListing_RealPriceChangeStillLands(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	const chatID = int64(1)
	rec := storage.ListingRecord{
		Token:       "dropping-car",
		ChatID:      chatID,
		SearchID:    1,
		Price:       75000,
		FirstSeenAt: time.Now(),
	}
	if err := store.SaveListing(ctx, rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	dropped := rec
	dropped.Price = 68000 // seller cut the price
	if err := store.SaveListing(ctx, dropped); err != nil {
		t.Fatalf("save drop: %v", err)
	}

	got, err := store.GetListing(ctx, chatID, rec.Token)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Price != 68000 {
		t.Fatalf("price drop did not land: got %d, want 68000", got.Price)
	}
}

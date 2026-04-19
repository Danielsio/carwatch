package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestClaimNew(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	isNew, err := store.ClaimNew(ctx, "token1", "search1")
	if err != nil {
		t.Fatalf("ClaimNew: %v", err)
	}
	if !isNew {
		t.Error("expected token1 to be new")
	}

	isNew, err = store.ClaimNew(ctx, "token1", "search1")
	if err != nil {
		t.Fatalf("ClaimNew duplicate: %v", err)
	}
	if isNew {
		t.Error("expected token1 to not be new on second claim")
	}
}

func TestReleaseClaim(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_, _ = store.ClaimNew(ctx, "token1", "search1")

	if err := store.ReleaseClaim(ctx, "token1"); err != nil {
		t.Fatalf("ReleaseClaim: %v", err)
	}

	isNew, err := store.ClaimNew(ctx, "token1", "search1")
	if err != nil {
		t.Fatalf("ClaimNew after release: %v", err)
	}
	if !isNew {
		t.Error("expected token1 to be new after release")
	}
}

func TestPrune(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_, _ = store.ClaimNew(ctx, "old-token", "search1")

	pruned, err := store.Prune(ctx, 0)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	isNew, _ := store.ClaimNew(ctx, "old-token", "search1")
	if !isNew {
		t.Error("expected old-token to be claimable after prune")
	}
}

func TestPruneKeepsRecent(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_, _ = store.ClaimNew(ctx, "recent-token", "search1")

	pruned, err := store.Prune(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned (recent), got %d", pruned)
	}
}

func TestNotificationQueue(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	if err := store.EnqueueNotification(ctx, "+972111", "search1", "hello"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := store.EnqueueNotification(ctx, "+972222", "search2", "world"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	pending, err := store.PendingNotifications(ctx)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}

	if err := store.AckNotification(ctx, pending[0].ID); err != nil {
		t.Fatalf("ack: %v", err)
	}

	remaining, _ := store.PendingNotifications(ctx)
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestRecordPrice(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_, changed, err := store.RecordPrice(ctx, "token1", 100000)
	if err != nil {
		t.Fatalf("record price: %v", err)
	}
	if changed {
		t.Error("first price should not be a change")
	}

	oldPrice, changed, err := store.RecordPrice(ctx, "token1", 90000)
	if err != nil {
		t.Fatalf("record price: %v", err)
	}
	if !changed {
		t.Error("price drop should be detected")
	}
	if oldPrice != 100000 {
		t.Errorf("old price = %d, want 100000", oldPrice)
	}

	_, changed, _ = store.RecordPrice(ctx, "token1", 95000)
	if changed {
		t.Error("price increase should not be detected as change")
	}
}

func TestSaveAndListListings(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	err = store.SaveListing(ctx, storage.ListingRecord{
		Token:        "abc",
		SearchName:   "test",
		Manufacturer: "Mazda",
		Model:        "3",
		Year:         2021,
		Price:        95000,
		Km:           85000,
		Hand:         2,
		City:         "Tel Aviv",
		PageLink:     "https://example.com",
	})
	if err != nil {
		t.Fatalf("save listing: %v", err)
	}

	listings, err := store.ListListings(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].Manufacturer != "Mazda" {
		t.Errorf("manufacturer = %q", listings[0].Manufacturer)
	}
}

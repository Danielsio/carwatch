package sqlite

import (
	"context"
	"testing"
	"time"
)

func TestSetUserTier(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "user1"); err != nil {
		t.Fatal(err)
	}

	// Default tier is free
	user, err := store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier != "free" {
		t.Fatalf("expected free tier, got: %s", user.Tier)
	}

	// Set to premium
	expires := time.Now().Add(30 * 24 * time.Hour)
	if err := store.SetUserTier(ctx, 100, "premium", expires); err != nil {
		t.Fatal(err)
	}

	user, err = store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier != "premium" {
		t.Fatalf("expected premium tier, got: %s", user.Tier)
	}
	if user.TierExpires.IsZero() {
		t.Fatal("expected non-zero expiry")
	}
}

func TestGrantTrial(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "user1"); err != nil {
		t.Fatal(err)
	}

	if err := store.GrantTrial(ctx, 100, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}

	user, err := store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier != "premium" {
		t.Fatalf("expected premium tier after trial, got: %s", user.Tier)
	}
	if !user.TrialUsed {
		t.Fatal("expected trial_used to be true")
	}
	if user.TierExpires.Before(time.Now()) {
		t.Fatal("expected future expiry")
	}
}

func TestListExpiredPremium(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create two users
	if err := store.UpsertUser(ctx, 100, "user1"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertUser(ctx, 200, "user2"); err != nil {
		t.Fatal(err)
	}

	// User 100: expired premium
	past := time.Now().Add(-1 * time.Hour)
	if err := store.SetUserTier(ctx, 100, "premium", past); err != nil {
		t.Fatal(err)
	}

	// User 200: active premium
	future := time.Now().Add(30 * 24 * time.Hour)
	if err := store.SetUserTier(ctx, 200, "premium", future); err != nil {
		t.Fatal(err)
	}

	expired, err := store.ListExpiredPremium(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired, got %d", len(expired))
	}
	if expired[0].ChatID != 100 {
		t.Fatalf("expected chat_id 100, got %d", expired[0].ChatID)
	}
}

func TestListExpiredPremiumExcludesFree(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "user1"); err != nil {
		t.Fatal(err)
	}

	// Free user with default zero expiry should not appear
	expired, err := store.ListExpiredPremium(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 0 {
		t.Fatalf("expected 0 expired, got %d", len(expired))
	}
}


package sqlite

import (
	"context"
	"testing"
	"time"
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

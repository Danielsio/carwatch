package sqlite

import (
	"context"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	seen, err := store.HasSeen(ctx, "token1")
	if err != nil {
		t.Fatalf("HasSeen: %v", err)
	}
	if seen {
		t.Error("expected token1 to not be seen")
	}

	if err := store.MarkSeen(ctx, "token1", "search1"); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}

	seen, err = store.HasSeen(ctx, "token1")
	if err != nil {
		t.Fatalf("HasSeen after mark: %v", err)
	}
	if !seen {
		t.Error("expected token1 to be seen")
	}

	if err := store.MarkSeen(ctx, "token1", "search1"); err != nil {
		t.Fatalf("MarkSeen duplicate: %v", err)
	}
}

func TestStore_Prune(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	if err := store.MarkSeen(ctx, "old-token", "search1"); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}

	pruned, err := store.Prune(ctx, 0)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	seen, _ := store.HasSeen(ctx, "old-token")
	if seen {
		t.Error("expected old-token to be pruned")
	}
}

func TestStore_PruneKeepsRecent(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	if err := store.MarkSeen(ctx, "recent-token", "search1"); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}

	pruned, err := store.Prune(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned (recent), got %d", pruned)
	}
}

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestPostgres_UserState(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if err := store.UpdateUserState(ctx, 100, "awaiting_price", `{"step":2}`); err != nil {
		t.Fatalf("UpdateUserState: %v", err)
	}
	u, err := store.GetUser(ctx, 100)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.State != "awaiting_price" || u.StateData != `{"step":2}` {
		t.Errorf("state = %q/%q", u.State, u.StateData)
	}
}

func TestPostgres_LastSeenAt(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	// Before any update, GetLastSeenAt falls back to created_at (non-zero).
	first, err := store.GetLastSeenAt(ctx, 100)
	if err != nil {
		t.Fatalf("GetLastSeenAt: %v", err)
	}
	if first.IsZero() {
		t.Fatal("expected created_at fallback, got zero time")
	}

	time.Sleep(5 * time.Millisecond)
	if err := store.UpdateLastSeenAt(ctx, 100); err != nil {
		t.Fatalf("UpdateLastSeenAt: %v", err)
	}
	second, _ := store.GetLastSeenAt(ctx, 100)
	if !second.After(first) {
		t.Errorf("last_seen_at did not advance: %v -> %v", first, second)
	}
}

func TestPostgres_UserTierAndTrial(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// SetUserTier on a missing user reports ErrNotFound.
	if err := store.SetUserTier(ctx, 100, "premium", time.Now().Add(time.Hour)); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("SetUserTier missing user = %v, want ErrNotFound", err)
	}

	seedPgUser(t, store, 100)
	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	if err := store.SetUserTier(ctx, 100, "premium", expires); err != nil {
		t.Fatalf("SetUserTier: %v", err)
	}
	u, _ := store.GetUser(ctx, 100)
	if u.Tier != "premium" || u.TierExpires.UTC().Truncate(time.Second) != expires {
		t.Errorf("tier = %q expires = %v, want premium/%v", u.Tier, u.TierExpires, expires)
	}

	// GrantTrial is one-shot: the second attempt is ErrNotFound (trial_used guard).
	seedPgUser(t, store, 200)
	if err := store.GrantTrial(ctx, 200, 14*24*time.Hour); err != nil {
		t.Fatalf("GrantTrial: %v", err)
	}
	u, _ = store.GetUser(ctx, 200)
	if u.Tier != "premium" || !u.TrialUsed {
		t.Errorf("after trial: tier=%q trial_used=%v", u.Tier, u.TrialUsed)
	}
	if err := store.GrantTrial(ctx, 200, 14*24*time.Hour); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("second GrantTrial = %v, want ErrNotFound", err)
	}
}

func TestPostgres_ListExpiredPremium(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	seedPgUser(t, store, 100) // expired premium
	seedPgUser(t, store, 200) // active premium
	seedPgUser(t, store, 300) // lifetime premium (epoch sentinel)
	seedPgUser(t, store, 400) // free, untouched

	if err := store.SetUserTier(ctx, 100, "premium", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserTier(ctx, 200, "premium", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Epoch expiry is the "never expires" sentinel and must be excluded.
	if err := store.SetUserTier(ctx, 300, "premium", time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	expired, err := store.ListExpiredPremium(ctx)
	if err != nil {
		t.Fatalf("ListExpiredPremium: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected exactly 1 expired premium, got %d (%+v)", len(expired), expired)
	}
	if expired[0].ChatID != 100 {
		t.Errorf("expired user = %d, want 100", expired[0].ChatID)
	}
}

func TestPostgres_ReactivateUsersWithSearches(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if _, err := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "s", Manufacturer: 1, Model: 1}); err != nil {
		t.Fatalf("CreateSearch: %v", err)
	}
	if err := store.SetUserActive(ctx, 100, false); err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}

	n, err := store.ReactivateUsersWithSearches(ctx)
	if err != nil {
		t.Fatalf("ReactivateUsersWithSearches: %v", err)
	}
	if n != 1 {
		t.Errorf("reactivated = %d, want 1", n)
	}
	u, _ := store.GetUser(ctx, 100)
	if !u.Active {
		t.Error("user should be active again")
	}
}

func TestPostgres_LinkTelegramToWeb(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100) // telegram user

	webID, err := store.UpsertWebUser(ctx, "firebase-uid-1", "user@example.com")
	if err != nil {
		t.Fatalf("UpsertWebUser: %v", err)
	}

	// No link yet.
	if linked, err := store.GetLinkedTelegramUser(ctx, webID); err != nil || linked != nil {
		t.Fatalf("pre-link GetLinkedTelegramUser = %v, %v", linked, err)
	}

	if err := store.LinkTelegramToWeb(ctx, 100, webID); err != nil {
		t.Fatalf("LinkTelegramToWeb: %v", err)
	}
	linked, err := store.GetLinkedTelegramUser(ctx, webID)
	if err != nil {
		t.Fatalf("GetLinkedTelegramUser: %v", err)
	}
	if linked == nil || linked.ChatID != 100 {
		t.Fatalf("linked telegram user = %+v, want chat 100", linked)
	}

	// Linking an unknown telegram user is ErrNotFound.
	if err := store.LinkTelegramToWeb(ctx, 999, webID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("link unknown telegram = %v, want ErrNotFound", err)
	}
}

func TestPostgres_CountSearches(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if n, err := store.CountSearches(ctx, 100); err != nil || n != 0 {
		t.Fatalf("empty CountSearches = %d, %v", n, err)
	}

	id1, _ := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "a", Manufacturer: 1, Model: 1})
	if _, err := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "b", Manufacturer: 1, Model: 2}); err != nil {
		t.Fatal(err)
	}
	if n, _ := store.CountSearches(ctx, 100); n != 2 {
		t.Errorf("CountSearches = %d, want 2", n)
	}

	// Inactive searches are excluded from the count.
	if err := store.SetSearchActive(ctx, id1, 100, false); err != nil {
		t.Fatalf("SetSearchActive: %v", err)
	}
	if n, _ := store.CountSearches(ctx, 100); n != 1 {
		t.Errorf("CountSearches after deactivate = %d, want 1", n)
	}
}

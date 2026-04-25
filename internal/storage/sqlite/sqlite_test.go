package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func seedUser(t *testing.T, store *Store, chatID int64) {
	t.Helper()
	if err := store.UpsertUser(context.Background(), chatID, "testuser"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

// --- User Tests ---

func TestUpsertUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "alice"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	u, err := store.GetUser(ctx, 100)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u == nil {
		t.Fatal("user should exist")
	}
	if u.Username != "alice" || u.State != "idle" || !u.Active {
		t.Errorf("user = %+v", u)
	}

	if err := store.UpsertUser(ctx, 100, "alice_new"); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	u, _ = store.GetUser(ctx, 100)
	if u.Username != "alice_new" {
		t.Errorf("username should update on upsert, got %q", u.Username)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	store := newTestStore(t)
	u, err := store.GetUser(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != nil {
		t.Error("expected nil for nonexistent user")
	}
}

func TestUpdateUserState(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.UpdateUserState(ctx, 100, "ask_manufacturer", `{"step":1}`); err != nil {
		t.Fatalf("update state: %v", err)
	}

	u, _ := store.GetUser(ctx, 100)
	if u.State != "ask_manufacturer" || u.StateData != `{"step":1}` {
		t.Errorf("state = %q, data = %q", u.State, u.StateData)
	}
}

func TestListActiveUsers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	seedUser(t, store, 100)
	seedUser(t, store, 200)
	_ = store.SetUserActive(ctx, 200, false)

	users, err := store.ListActiveUsers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 1 || users[0].ChatID != 100 {
		t.Errorf("expected 1 active user (100), got %d", len(users))
	}
}

func TestCountUsers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	seedUser(t, store, 100)
	seedUser(t, store, 200)
	_ = store.SetUserActive(ctx, 200, false)

	count, err := store.CountUsers(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

// --- Search Tests ---

func TestCreateAndListSearches(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       100,
		Name:         "mazda3-2.0",
		Manufacturer: 27,
		Model:        10332,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     150000,
		EngineMinCC:  1800,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero search ID")
	}

	searches, err := store.ListSearches(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	s := searches[0]
	if s.Name != "mazda3-2.0" || s.Manufacturer != 27 || s.PriceMax != 150000 {
		t.Errorf("search = %+v", s)
	}
}

func TestGetSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	id, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "test", Manufacturer: 1, Model: 1,
	})

	s, err := store.GetSearch(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s == nil || s.Name != "test" {
		t.Errorf("search = %+v", s)
	}

	s, err = store.GetSearch(ctx, 999)
	if err != nil {
		t.Fatalf("get nonexistent: %v", err)
	}
	if s != nil {
		t.Error("expected nil for nonexistent search")
	}
}

func TestDeleteSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	id, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "test", Manufacturer: 1, Model: 1,
	})

	if err := store.DeleteSearch(ctx, id, 100); err != nil {
		t.Fatalf("delete: %v", err)
	}

	searches, _ := store.ListSearches(ctx, 100)
	if len(searches) != 0 {
		t.Error("search should be deleted")
	}
}

func TestDeleteSearch_WrongOwner(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	id, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "test", Manufacturer: 1, Model: 1,
	})

	_ = store.DeleteSearch(ctx, id, 200)

	s, _ := store.GetSearch(ctx, id)
	if s == nil {
		t.Error("search should NOT be deleted by wrong owner")
	}
}

func TestSetSearchActive(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	id, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "test", Manufacturer: 1, Model: 1,
	})

	_ = store.SetSearchActive(ctx, id, false)
	s, _ := store.GetSearch(ctx, id)
	if s.Active {
		t.Error("search should be inactive")
	}

	_ = store.SetSearchActive(ctx, id, true)
	s, _ = store.GetSearch(ctx, id)
	if !s.Active {
		t.Error("search should be active again")
	}
}

func TestListAllActiveSearches(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	_, _ = store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "a", Manufacturer: 27, Model: 10332})
	_, _ = store.CreateSearch(ctx, storage.Search{ChatID: 200, Name: "b", Manufacturer: 27, Model: 10332})

	id3, _ := store.CreateSearch(ctx, storage.Search{ChatID: 200, Name: "c", Manufacturer: 1, Model: 1})
	_ = store.SetSearchActive(ctx, id3, false)

	searches, err := store.ListAllActiveSearches(ctx)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(searches) != 2 {
		t.Errorf("expected 2 active searches, got %d", len(searches))
	}
}

func TestCountSearches(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	_, _ = store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "a", Manufacturer: 1, Model: 1})
	_, _ = store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "b", Manufacturer: 2, Model: 2})

	count, _ := store.CountSearches(ctx, 100)
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	total, _ := store.CountAllSearches(ctx)
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
}

func TestCreateSearch_AssignsUserSeq(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	id1, _ := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "first", Manufacturer: 1, Model: 1})
	id2, _ := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "second", Manufacturer: 2, Model: 2})
	id3, _ := store.CreateSearch(ctx, storage.Search{ChatID: 200, Name: "other-user", Manufacturer: 1, Model: 1})

	s1, _ := store.GetSearch(ctx, id1)
	s2, _ := store.GetSearch(ctx, id2)
	s3, _ := store.GetSearch(ctx, id3)

	if s1.UserSeq != 1 {
		t.Errorf("first search UserSeq = %d, want 1", s1.UserSeq)
	}
	if s2.UserSeq != 2 {
		t.Errorf("second search UserSeq = %d, want 2", s2.UserSeq)
	}
	if s3.UserSeq != 1 {
		t.Errorf("other user's first search UserSeq = %d, want 1", s3.UserSeq)
	}
}

func TestGetSearchBySeq(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	_, _ = store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "first", Manufacturer: 1, Model: 1})
	_, _ = store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "second", Manufacturer: 2, Model: 2})

	s, err := store.GetSearchBySeq(ctx, 100, 2)
	if err != nil {
		t.Fatalf("get by seq: %v", err)
	}
	if s == nil || s.Name != "second" {
		t.Errorf("expected 'second', got %+v", s)
	}

	s, err = store.GetSearchBySeq(ctx, 100, 99)
	if err != nil {
		t.Fatalf("get nonexistent seq: %v", err)
	}
	if s != nil {
		t.Error("expected nil for nonexistent seq")
	}
}

// --- DedupStore (per-user) ---

func TestClaimNew_PerUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	isNew, err := store.ClaimNew(ctx, "token1", 100, 1)
	if err != nil || !isNew {
		t.Fatalf("first claim: new=%v, err=%v", isNew, err)
	}

	isNew, err = store.ClaimNew(ctx, "token1", 100, 1)
	if err != nil || isNew {
		t.Error("duplicate claim for same user should return false")
	}

	isNew, err = store.ClaimNew(ctx, "token1", 200, 1)
	if err != nil || !isNew {
		t.Error("same token for different user should be new")
	}
}

func TestReleaseClaim_PerUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	_, _ = store.ClaimNew(ctx, "token1", 100, 1)
	_, _ = store.ClaimNew(ctx, "token1", 200, 1)

	if err := store.ReleaseClaim(ctx, "token1", 100); err != nil {
		t.Fatalf("release: %v", err)
	}

	isNew, _ := store.ClaimNew(ctx, "token1", 100, 1)
	if !isNew {
		t.Error("released token should be claimable again for user 100")
	}

	isNew, _ = store.ClaimNew(ctx, "token1", 200, 1)
	if isNew {
		t.Error("user 200's claim should be unaffected")
	}
}

func TestPrune(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	_, _ = store.ClaimNew(ctx, "old-token", 100, 1)
	pruned, err := store.Prune(ctx, 0)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}
}

func TestPruneKeepsRecent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	_, _ = store.ClaimNew(ctx, "recent-token", 100, 1)
	pruned, err := store.Prune(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned (recent), got %d", pruned)
	}
}

// --- NotificationQueue ---

func TestNotificationQueue(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.EnqueueNotification(ctx, "100", "search1", "hello")
	_ = store.EnqueueNotification(ctx, "200", "search2", "world")

	pending, _ := store.PendingNotifications(ctx)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}

	_ = store.AckNotification(ctx, pending[0].ID)
	remaining, _ := store.PendingNotifications(ctx)
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestPruneNotifications(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.EnqueueNotification(ctx, "100", "search1", "hello")
	_ = store.EnqueueNotification(ctx, "200", "search2", "world")

	// Prune with zero duration removes all.
	pruned, err := store.PruneNotifications(ctx, 0)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 2 {
		t.Errorf("expected 2 pruned, got %d", pruned)
	}

	remaining, _ := store.PendingNotifications(ctx)
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining, got %d", len(remaining))
	}
}

func TestPruneNotifications_KeepsRecent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.EnqueueNotification(ctx, "100", "search1", "hello")

	pruned, err := store.PruneNotifications(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned (recent), got %d", pruned)
	}
}

// --- PriceTracker ---

func TestPrunePrices(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, _, _ = store.RecordPrice(ctx, "token1", 100000)
	_, _, _ = store.RecordPrice(ctx, "token1", 90000)

	pruned, err := store.PrunePrices(ctx, 0)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 2 {
		t.Errorf("expected 2 pruned, got %d", pruned)
	}
}

func TestPrunePrices_KeepsRecent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, _, _ = store.RecordPrice(ctx, "token1", 100000)

	pruned, err := store.PrunePrices(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned (recent), got %d", pruned)
	}
}

func TestRecordPrice(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, changed, _ := store.RecordPrice(ctx, "token1", 100000)
	if changed {
		t.Error("first price should not be a change")
	}

	oldPrice, changed, _ := store.RecordPrice(ctx, "token1", 90000)
	if !changed {
		t.Error("price drop should be detected")
	}
	if oldPrice != 100000 {
		t.Errorf("old price = %d, want 100000", oldPrice)
	}

	oldPrice, changed, _ = store.RecordPrice(ctx, "token1", 95000)
	if !changed {
		t.Error("price increase should be detected as change")
	}
	if oldPrice != 90000 {
		t.Errorf("old price = %d, want 90000", oldPrice)
	}
}

// --- ListingStore ---

// --- DigestStore ---

func TestSetAndGetDigestMode(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	// Default should be instant.
	mode, interval, err := store.GetDigestMode(ctx, 100)
	if err != nil {
		t.Fatalf("get digest mode: %v", err)
	}
	if mode != "instant" || interval != "6h" {
		t.Errorf("default mode=%q interval=%q, want instant/6h", mode, interval)
	}

	// Switch to digest mode.
	if err := store.SetDigestMode(ctx, 100, "digest", "12h"); err != nil {
		t.Fatalf("set digest mode: %v", err)
	}

	mode, interval, err = store.GetDigestMode(ctx, 100)
	if err != nil {
		t.Fatalf("get digest mode: %v", err)
	}
	if mode != "digest" || interval != "12h" {
		t.Errorf("mode=%q interval=%q, want digest/12h", mode, interval)
	}

	// Switch back to instant.
	if err := store.SetDigestMode(ctx, 100, "instant", "6h"); err != nil {
		t.Fatalf("set instant: %v", err)
	}
	mode, _, _ = store.GetDigestMode(ctx, 100)
	if mode != "instant" {
		t.Errorf("mode=%q, want instant", mode)
	}
}

func TestGetDigestMode_NonexistentUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mode, interval, err := store.GetDigestMode(ctx, 999)
	if err != nil {
		t.Fatalf("get digest mode: %v", err)
	}
	if mode != "instant" || interval != "6h" {
		t.Errorf("nonexistent user: mode=%q interval=%q, want instant/6h", mode, interval)
	}
}

func TestAddAndFlushDigest(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	// Add items.
	if err := store.AddDigestItem(ctx, 100, "listing 1"); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if err := store.AddDigestItem(ctx, 100, "listing 2"); err != nil {
		t.Fatalf("add item: %v", err)
	}

	// Flush.
	payloads, err := store.FlushDigest(ctx, 100)
	if err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("expected 2 payloads, got %d", len(payloads))
	}
	if payloads[0] != "listing 1" || payloads[1] != "listing 2" {
		t.Errorf("payloads = %v", payloads)
	}

	// Flush again should return empty.
	payloads, err = store.FlushDigest(ctx, 100)
	if err != nil {
		t.Fatalf("second flush: %v", err)
	}
	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads after flush, got %d", len(payloads))
	}
}

func TestFlushDigest_Empty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	payloads, err := store.FlushDigest(ctx, 100)
	if err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads, got %d", len(payloads))
	}
}

func TestPendingDigestUsers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	// No pending items.
	users, err := store.PendingDigestUsers(ctx)
	if err != nil {
		t.Fatalf("pending users: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}

	// Add items for two users.
	_ = store.AddDigestItem(ctx, 100, "item1")
	_ = store.AddDigestItem(ctx, 100, "item2")
	_ = store.AddDigestItem(ctx, 200, "item3")

	users, err = store.PendingDigestUsers(ctx)
	if err != nil {
		t.Fatalf("pending users: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestDigestLastFlushed(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	// Initial value should be epoch.
	ts, err := store.DigestLastFlushed(ctx, 100)
	if err != nil {
		t.Fatalf("last flushed: %v", err)
	}
	if ts.Year() != 1970 {
		t.Errorf("expected epoch, got %v", ts)
	}

	// Add and flush to update timestamp.
	_ = store.AddDigestItem(ctx, 100, "item")
	_, _ = store.FlushDigest(ctx, 100)

	ts, err = store.DigestLastFlushed(ctx, 100)
	if err != nil {
		t.Fatalf("last flushed after flush: %v", err)
	}

	if time.Since(ts) > 10*time.Second {
		t.Errorf("last flushed should be recent, got %v", ts)
	}
}

func TestDigestIsolation_BetweenUsers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	_ = store.AddDigestItem(ctx, 100, "user100-item")
	_ = store.AddDigestItem(ctx, 200, "user200-item")

	// Flush user 100 only.
	payloads, _ := store.FlushDigest(ctx, 100)
	if len(payloads) != 1 || payloads[0] != "user100-item" {
		t.Errorf("user 100 payloads = %v", payloads)
	}

	// User 200 should still have their item.
	payloads, _ = store.FlushDigest(ctx, 200)
	if len(payloads) != 1 || payloads[0] != "user200-item" {
		t.Errorf("user 200 payloads = %v", payloads)
	}
}

// --- ListingStore ---

func TestSaveAndListListings(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "abc", ChatID: 100, SearchName: "test", Manufacturer: "Mazda", Model: "3",
		Year: 2021, Price: 95000, Km: 85000, Hand: 2, City: "Tel Aviv",
		PageLink: "https://example.com",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	listings, _ := store.ListListings(ctx, 10)
	if len(listings) != 1 || listings[0].Manufacturer != "Mazda" {
		t.Errorf("listings = %+v", listings)
	}
}

func TestListUserListings(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	// Save listings per user — listing_history is now per-user via chat_id.
	_ = store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-a", ChatID: 100, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2020, Price: 100000,
	})
	_ = store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-b", ChatID: 100, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2020, Price: 100000,
	})
	_ = store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-c", ChatID: 200, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2020, Price: 100000,
	})

	// User 100 should see 2 listings.
	listings, err := store.ListUserListings(ctx, 100, 10, 0)
	if err != nil {
		t.Fatalf("list user listings: %v", err)
	}
	if len(listings) != 2 {
		t.Errorf("expected 2 listings for user 100, got %d", len(listings))
	}

	// User 200 should see 1 listing.
	listings, err = store.ListUserListings(ctx, 200, 10, 0)
	if err != nil {
		t.Fatalf("list user listings: %v", err)
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing for user 200, got %d", len(listings))
	}
}

func TestCountUserListings(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	// No listings yet.
	count, err := store.CountUserListings(ctx, 100)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	// Add some.
	for _, tok := range []string{"t1", "t2", "t3"} {
		_ = store.SaveListing(ctx, storage.ListingRecord{
			Token: tok, ChatID: 100, SearchName: "test",
			Manufacturer: "Mazda", Model: "3",
		})
	}

	count, err = store.CountUserListings(ctx, 100)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestListUserListings_Pagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	for i := range 5 {
		tok := "tok-" + string(rune('a'+i))
		_ = store.SaveListing(ctx, storage.ListingRecord{
			Token: tok, ChatID: 100, SearchName: "test",
			Manufacturer: "Test", Model: "Car",
			Price: 100000 + i*1000,
		})
	}

	// Page 1: limit 2, offset 0.
	page1, _ := store.ListUserListings(ctx, 100, 2, 0)
	if len(page1) != 2 {
		t.Errorf("page 1: expected 2 items, got %d", len(page1))
	}

	// Page 2: limit 2, offset 2.
	page2, _ := store.ListUserListings(ctx, 100, 2, 2)
	if len(page2) != 2 {
		t.Errorf("page 2: expected 2 items, got %d", len(page2))
	}

	// Page 3: limit 2, offset 4.
	page3, _ := store.ListUserListings(ctx, 100, 2, 4)
	if len(page3) != 1 {
		t.Errorf("page 3: expected 1 item, got %d", len(page3))
	}
}

// --- MarketStore Tests ---

func TestMarketListings(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	// Insert listings for two users with same token (should deduplicate)
	for _, chatID := range []int64{100, 200} {
		_ = store.SaveListing(ctx, storage.ListingRecord{
			Token: "tok1", ChatID: chatID, SearchName: "toyota-corolla",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020,
			Price: 100000, FirstSeenAt: time.Now(),
		})
	}
	// Another listing
	_ = store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok2", ChatID: 100, SearchName: "toyota-corolla",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021,
		Price: 110000, FirstSeenAt: time.Now(),
	})
	// Listing with empty manufacturer (should be excluded)
	_ = store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok3", ChatID: 100, SearchName: "test",
		Manufacturer: "", Model: "X", Year: 2020,
		Price: 50000, FirstSeenAt: time.Now(),
	})

	listings, err := store.MarketListings(ctx)
	if err != nil {
		t.Fatalf("MarketListings: %v", err)
	}
	// tok1 should appear once (deduplicated), tok2 once, tok3 excluded
	if len(listings) != 2 {
		t.Errorf("expected 2 deduplicated listings, got %d", len(listings))
	}
}

// --- DailyDigestStore Tests ---

func TestDailyDigest_SetAndGet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	// Default state
	enabled, digestTime, _, err := store.GetDailyDigest(ctx, 100)
	if err != nil {
		t.Fatalf("GetDailyDigest: %v", err)
	}
	if enabled {
		t.Error("expected daily digest disabled by default")
	}
	if digestTime != "09:00" {
		t.Errorf("expected default time 09:00, got %q", digestTime)
	}

	// Enable
	if err := store.SetDailyDigest(ctx, 100, true, "08:30"); err != nil {
		t.Fatalf("SetDailyDigest: %v", err)
	}

	enabled, digestTime, _, err = store.GetDailyDigest(ctx, 100)
	if err != nil {
		t.Fatalf("GetDailyDigest after set: %v", err)
	}
	if !enabled {
		t.Error("expected daily digest enabled")
	}
	if digestTime != "08:30" {
		t.Errorf("expected time 08:30, got %q", digestTime)
	}
}

func TestDailyDigest_ListUsers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	_ = store.SetDailyDigest(ctx, 100, true, "09:00")
	// User 200 not enabled

	users, err := store.ListDailyDigestUsers(ctx)
	if err != nil {
		t.Fatalf("ListDailyDigestUsers: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
	if len(users) > 0 && users[0].ChatID != 100 {
		t.Errorf("expected chatID 100, got %d", users[0].ChatID)
	}
}

func TestDailyDigest_UpdateLastSent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	_ = store.SetDailyDigest(ctx, 100, true, "09:00")

	if err := store.UpdateDailyDigestLastSent(ctx, 100); err != nil {
		t.Fatalf("UpdateDailyDigestLastSent: %v", err)
	}

	_, _, lastSent, err := store.GetDailyDigest(ctx, 100)
	if err != nil {
		t.Fatalf("GetDailyDigest: %v", err)
	}
	if time.Since(lastSent) > time.Minute {
		t.Errorf("expected lastSent to be recent, got %v", lastSent)
	}
}

func TestDailyDigest_NonexistentUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	enabled, digestTime, _, err := store.GetDailyDigest(ctx, 999)
	if err != nil {
		t.Fatalf("GetDailyDigest nonexistent: %v", err)
	}
	if enabled {
		t.Error("expected disabled for nonexistent user")
	}
	if digestTime != "09:00" {
		t.Errorf("expected default time, got %q", digestTime)
	}
}

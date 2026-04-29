package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// --- New / Close edge cases ---

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/proc/nonexistent/path/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestNew_MemoryDB(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestDB_Accessor(t *testing.T) {
	store := newTestStore(t)
	db := store.DB()
	if db == nil {
		t.Fatal("DB() should not return nil")
	}
	var result int
	if err := db.QueryRow("SELECT 1").Scan(&result); err != nil {
		t.Fatalf("query via DB(): %v", err)
	}
	if result != 1 {
		t.Errorf("result = %d, want 1", result)
	}
}

func TestCheckpoint(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Checkpoint(); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
}

func TestClose_OnDisk(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "close-test.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.UpsertUser(context.Background(), 1, "test"); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("db file should exist after close")
	}
}

// --- RecordPrice edge cases ---

func TestRecordPrice_SamePrice(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "price-same", ChatID: 100, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err := store.RecordPrice(ctx, "price-same", 100000)
	if err != nil {
		t.Fatal(err)
	}

	old, changed, err := store.RecordPrice(ctx, "price-same", 100000)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("same price should not be marked as changed")
	}
	if old != 100000 {
		t.Errorf("old = %d, want 100000", old)
	}
}

func TestRecordPrice_PriceChange(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "price-chg", ChatID: 100, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err := store.RecordPrice(ctx, "price-chg", 100000)
	if err != nil {
		t.Fatal(err)
	}

	old, changed, err := store.RecordPrice(ctx, "price-chg", 90000)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("different price should be marked as changed")
	}
	if old != 100000 {
		t.Errorf("old = %d, want 100000", old)
	}
}

func TestRecordPrice_FirstRecord(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, changed, err := store.RecordPrice(ctx, "new-token", 50000)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("first record should not be changed")
	}
}

// --- PrunePrices ---

func TestPrunePrices_OldRecords(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, _, _ = store.RecordPrice(ctx, "prune-tok", 100000)

	db := store.DB()
	_, err := db.ExecContext(ctx,
		"UPDATE price_history SET observed_at = datetime('now', '-100 days') WHERE token = 'prune-tok'")
	if err != nil {
		t.Fatal(err)
	}

	pruned, err := store.PrunePrices(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if pruned < 1 {
		t.Errorf("expected at least 1 pruned, got %d", pruned)
	}
}

// --- PruneNotifications ---

func TestPruneNotifications_OldRecords(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.EnqueueNotification(ctx, "100", "search1", "payload1"); err != nil {
		t.Fatal(err)
	}

	db := store.DB()
	_, err := db.ExecContext(ctx,
		"UPDATE pending_notifications SET created_at = datetime('now', '-100 days') WHERE search_name = 'search1'")
	if err != nil {
		t.Fatal(err)
	}

	pruned, err := store.PruneNotifications(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if pruned < 1 {
		t.Errorf("expected at least 1 pruned, got %d", pruned)
	}
}

// --- UpdateSearch ErrNotFound ---

func TestUpdateSearch_WrongOwner_ReturnsErrNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "s1", Source: "yad2",
		Manufacturer: 19, Model: 10226, YearMin: 2020, YearMax: 2024,
		Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.UpdateSearch(ctx, storage.Search{
		ID: id, ChatID: 200, Name: "s1", Source: "yad2",
		Manufacturer: 19, Model: 10226, YearMin: 2020, YearMax: 2025,
	})
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound for wrong owner update, got %v", err)
	}
}

// --- DeleteSearch cascade ---

func TestDeleteSearch_CascadeCleanup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "cascade-test", Source: "yad2",
		Manufacturer: 19, Model: 10226, YearMin: 2020, YearMax: 2024,
		Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "cascade-tok", ChatID: 100, SearchName: "cascade-test",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.EnqueueNotification(ctx, "100", "cascade-test", "payload"); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteSearch(ctx, id, 100); err != nil {
		t.Fatal(err)
	}

	listings, err := store.ListSearchListings(ctx, 100, "cascade-test", 100, 0, "newest")
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 0 {
		t.Errorf("expected 0 listings after cascade delete, got %d", len(listings))
	}

	pending, err := store.PendingNotifications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pending {
		if p.SearchName == "cascade-test" {
			t.Error("pending notifications should be cascade deleted")
		}
	}
}

// --- SetUserTier / GrantTrial ---

func TestSetUserTier_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.SetUserTier(ctx, 99999, "premium", time.Now().Add(24*time.Hour))
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGrantTrial_Success(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	err := store.GrantTrial(ctx, 100, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	u, err := store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if u.Tier != "premium" {
		t.Errorf("tier = %q, want premium", u.Tier)
	}
	if !u.TrialUsed {
		t.Error("trial_used should be true")
	}
}

func TestGrantTrial_AlreadyUsed(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.GrantTrial(ctx, 100, 24*time.Hour); err != nil {
		t.Fatal(err)
	}

	err := store.GrantTrial(ctx, 100, 24*time.Hour)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound for double trial, got %v", err)
	}
}

func TestGrantTrial_UserNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.GrantTrial(ctx, 99999, 24*time.Hour)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- ListExpiredPremium ---

func TestListExpiredPremium_Mixed(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	past := time.Now().Add(-24 * time.Hour)
	if err := store.SetUserTier(ctx, 100, "premium", past); err != nil {
		t.Fatal(err)
	}

	future := time.Now().Add(24 * time.Hour)
	if err := store.SetUserTier(ctx, 200, "premium", future); err != nil {
		t.Fatal(err)
	}

	expired, err := store.ListExpiredPremium(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0].ChatID != 100 {
		t.Errorf("expected only user 100 expired, got %v", expired)
	}
}

// --- CountUsers / CountSearches / CountAllSearches ---

func TestCountUsers_ActiveOnly(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)
	_ = store.SetUserActive(ctx, 200, false)

	count, err := store.CountUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("active users = %d, want 1", count)
	}
}

func TestCountAllSearches(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	_, _ = store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "s1", Source: "yad2",
		Manufacturer: 19, Model: 10226, Active: true,
	})
	_, _ = store.CreateSearch(ctx, storage.Search{
		ChatID: 200, Name: "s2", Source: "yad2",
		Manufacturer: 8, Model: 10061, Active: true,
	})

	count, err := store.CountAllSearches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("all searches = %d, want 2", count)
	}
}

// --- GetSearchByShareToken ---

func TestGetSearchByShareToken_Coverage(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "shared-s", Source: "yad2",
		Manufacturer: 19, Model: 10226, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	search, err := store.GetSearch(ctx, id)
	if err != nil || search == nil {
		t.Fatal("search should exist")
	}

	if search.ShareToken == "" {
		t.Fatal("share token should be auto-generated")
	}

	found, err := store.GetSearchByShareToken(ctx, search.ShareToken)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != id {
		t.Errorf("GetSearchByShareToken should find the search")
	}

	notFound, err := store.GetSearchByShareToken(ctx, "nonexistent-token")
	if err != nil {
		t.Fatal(err)
	}
	if notFound != nil {
		t.Error("should return nil for unknown share token")
	}
}

// --- GetSearchBySeq ---

func TestGetSearchBySeq_Coverage(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	_, _ = store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "seq-s1", Source: "yad2",
		Manufacturer: 19, Model: 10226, Active: true,
	})

	found, err := store.GetSearchBySeq(ctx, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Name != "seq-s1" {
		t.Error("should find search by user_seq=1")
	}

	notFound, err := store.GetSearchBySeq(ctx, 100, 99)
	if err != nil {
		t.Fatal(err)
	}
	if notFound != nil {
		t.Error("should return nil for unknown seq")
	}
}

// --- SetUserLanguage ---

func TestSetUserLanguage_Coverage(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.SetUserLanguage(ctx, 100, "en"); err != nil {
		t.Fatal(err)
	}
	u, _ := store.GetUser(ctx, 100)
	if u.Language != "en" {
		t.Errorf("language = %q, want en", u.Language)
	}
}

// --- NewListingsSince with offset ---

func TestNewListingsSince_Offset(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	since := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: "nls-" + string(rune('a'+i)), ChatID: 100, SearchName: "s1",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
			FirstSeenAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	listings, err := store.NewListingsSince(ctx, 100, since, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 2 {
		t.Errorf("expected 2 listings at offset 2, got %d", len(listings))
	}

	count, err := store.CountNewListingsSince(ctx, 100, since)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("total count = %d, want 5", count)
	}
}

// --- GetLastSeenAt falls back to created_at ---

func TestGetLastSeenAt_FallsBackToCreatedAt(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	ts, err := store.GetLastSeenAt(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Error("should fall back to created_at, not zero")
	}
}

func TestGetLastSeenAt_UserNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetLastSeenAt(ctx, 99999)
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

// --- ListAllActiveSearches ---

func TestListAllActiveSearches_ExcludesPaused(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	_, _ = store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "active-s", Source: "yad2",
		Manufacturer: 19, Model: 10226, Active: true,
	})
	id2, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 200, Name: "paused-s", Source: "yad2",
		Manufacturer: 8, Model: 10061, Active: true,
	})
	_ = store.SetSearchActive(ctx, id2, 200, false)

	searches, err := store.ListAllActiveSearches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(searches) != 1 {
		t.Errorf("expected 1 active search, got %d", len(searches))
	}
}

// --- DBFileSize in-memory ---

func TestDBFileSize_InMemory_ReturnsZero(t *testing.T) {
	store := newTestStore(t)
	size, err := store.DBFileSize()
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 {
		t.Errorf("in-memory db should have size 0, got %d", size)
	}
}

// --- New with query params in path ---

func TestNew_WithQueryParams(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db?mode=rwc")
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("expected success with query params: %v", err)
	}
	store.Close()
}

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// testStore returns a *Store connected to a real Postgres instance.
// Skips the test when TEST_POSTGRES_DSN is not set.
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; skipping Postgres integration test")
	}
	migrationsPath := os.Getenv("TEST_POSTGRES_MIGRATIONS")
	if migrationsPath == "" {
		migrationsPath = "../../../migrations"
	}
	store, err := New(dsn, migrationsPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() {
		db := store.DB()
		tables := []string{
			"listing_user_seen", "pending_digest",
			"saved_listings", "hidden_listings", "listing_history",
			"seen_listings", "price_history", "link_tokens",
			"daily_digest", "searches", "price_list_cache", "users",
		}
		for _, tbl := range tables {
			_, _ = db.Exec("DELETE FROM " + tbl)
		}
		_ = store.Close()
	})
	return store
}

func seedPgUser(t *testing.T, store *Store, chatID int64) {
	t.Helper()
	if err := store.UpsertUser(context.Background(), chatID, "testuser"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func ptrBoolPg(b bool) *bool { return &b }

// ---------------------------------------------------------------------------
// User CRUD
// ---------------------------------------------------------------------------

func TestPostgres_UserCRUD(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Create
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

	// Upsert updates username
	if err := store.UpsertUser(ctx, 100, "alice_new"); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	u, _ = store.GetUser(ctx, 100)
	if u.Username != "alice_new" {
		t.Errorf("username should update on upsert, got %q", u.Username)
	}

	// GetUser not found
	u, err = store.GetUser(ctx, 999)
	if err != nil {
		t.Fatalf("get nonexistent: %v", err)
	}
	if u != nil {
		t.Error("expected nil for nonexistent user")
	}

	// SetUserActive
	seedPgUser(t, store, 200)
	if err := store.SetUserActive(ctx, 200, false); err != nil {
		t.Fatalf("set inactive: %v", err)
	}
	users, err := store.ListActiveUsers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 1 || users[0].ChatID != 100 {
		t.Errorf("expected 1 active user (100), got %d", len(users))
	}

	// CountUsers counts only active
	count, err := store.CountUsers(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// UpsertUser preserves admin-set active status (does not force reactivate)
	if err := store.UpsertUser(ctx, 200, "reactivated"); err != nil {
		t.Fatalf("upsert reactivate: %v", err)
	}
	u, _ = store.GetUser(ctx, 200)
	if u.Active {
		t.Error("UpsertUser should not override admin deactivation")
	}

	// SetUserLanguage
	if err := store.SetUserLanguage(ctx, 100, "en"); err != nil {
		t.Fatalf("SetUserLanguage: %v", err)
	}
	u, _ = store.GetUser(ctx, 100)
	if u.Language != "en" {
		t.Errorf("language = %q, want en", u.Language)
	}
}

// ---------------------------------------------------------------------------
// Search CRUD
// ---------------------------------------------------------------------------

func TestPostgres_SearchCRUD(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	// Create
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

	// List
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

	// GetSearch
	s2, err := store.GetSearch(ctx, id, 100)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s2 == nil || s2.Name != "mazda3-2.0" {
		t.Errorf("search = %+v", s2)
	}

	// GetSearch wrong owner
	s2, err = store.GetSearch(ctx, id, 999)
	if err != nil {
		t.Fatalf("get wrong owner: %v", err)
	}
	if s2 != nil {
		t.Error("expected nil when chatID does not match")
	}

	// UpdateSearch
	err = store.UpdateSearch(ctx, storage.Search{
		ID: id, ChatID: 100, Name: "updated", Source: "yad2",
		Manufacturer: 27, Model: 10332, YearMin: 2020, YearMax: 2025, PriceMax: 180000,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	s3, _ := store.GetSearch(ctx, id, 100)
	if s3.Name != "updated" || s3.PriceMax != 180000 {
		t.Errorf("after update: name=%q price_max=%d", s3.Name, s3.PriceMax)
	}

	// UpdateSearch not found
	err = store.UpdateSearch(ctx, storage.Search{ID: 999, ChatID: 100, Name: "x"})
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// SetSearchActive
	if err := store.SetSearchActive(ctx, id, 100, false); err != nil {
		t.Fatalf("set inactive: %v", err)
	}
	s4, _ := store.GetSearch(ctx, id, 100)
	if s4.Active {
		t.Error("search should be inactive")
	}
	if err := store.SetSearchActive(ctx, id, 100, true); err != nil {
		t.Fatalf("set active: %v", err)
	}
	s4, _ = store.GetSearch(ctx, id, 100)
	if !s4.Active {
		t.Error("search should be active again")
	}

	// SetSearchActive wrong owner
	if err := store.SetSearchActive(ctx, id, 999, false); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong owner, got: %v", err)
	}

	// Delete
	if err := store.DeleteSearch(ctx, id, 100); err != nil {
		t.Fatalf("delete: %v", err)
	}
	searches, _ = store.ListSearches(ctx, 100)
	if len(searches) != 0 {
		t.Error("search should be deleted")
	}

	// Delete wrong owner
	seedPgUser(t, store, 300)
	id2, _ := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "owned", Manufacturer: 1, Model: 1})
	if err := store.DeleteSearch(ctx, id2, 300); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong-owner delete, got: %v", err)
	}
}

func TestPostgres_SearchUserSeq(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	id1, _ := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "first", Manufacturer: 1, Model: 1})
	id2, _ := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "second", Manufacturer: 2, Model: 2})
	id3, _ := store.CreateSearch(ctx, storage.Search{ChatID: 200, Name: "other-user", Manufacturer: 1, Model: 1})

	s1, _ := store.GetSearch(ctx, id1, 100)
	s2, _ := store.GetSearch(ctx, id2, 100)
	s3, _ := store.GetSearch(ctx, id3, 200)

	if s1.UserSeq != 1 {
		t.Errorf("first search UserSeq = %d, want 1", s1.UserSeq)
	}
	if s2.UserSeq != 2 {
		t.Errorf("second search UserSeq = %d, want 2", s2.UserSeq)
	}
	if s3.UserSeq != 1 {
		t.Errorf("other user's first search UserSeq = %d, want 1", s3.UserSeq)
	}

	// GetSearchBySeq
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

func TestPostgres_SearchShareToken(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	id, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "shared", Source: "yad2",
		Manufacturer: 27, Model: 10332,
	})

	search, _ := store.GetSearch(ctx, id, 100)
	if search.ShareToken == "" {
		t.Fatal("expected non-empty share token")
	}

	found, err := store.GetSearchByShareToken(ctx, search.ShareToken)
	if err != nil {
		t.Fatalf("GetSearchByShareToken: %v", err)
	}
	if found == nil || found.ID != id {
		t.Errorf("expected ID %d, got %+v", id, found)
	}

	// Not found
	found, err = store.GetSearchByShareToken(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSearchByShareToken nonexistent: %v", err)
	}
	if found != nil {
		t.Error("expected nil for nonexistent token")
	}
}

// ---------------------------------------------------------------------------
// Listing save, query, pagination
// ---------------------------------------------------------------------------

func TestPostgres_ListingSaveAndQuery(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	searchID, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "mazda3", Source: "yad2",
		Manufacturer: 8, Model: 10061,
	})

	// SaveListing
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-1", ChatID: 100, SearchID: searchID, SearchName: "mazda3",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 95000,
		Km: 50000, Hand: 2, City: "Tel Aviv",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	// GetListing
	l, err := store.GetListing(ctx, 100, "tok-1")
	if err != nil {
		t.Fatalf("get listing: %v", err)
	}
	if l == nil || l.Manufacturer != "Mazda" || l.Price != 95000 {
		t.Errorf("listing = %+v", l)
	}

	// GetListing not found
	l, _ = store.GetListing(ctx, 100, "nonexistent")
	if l != nil {
		t.Error("expected nil for nonexistent token")
	}

	// GetListing wrong owner
	seedPgUser(t, store, 200)
	l, _ = store.GetListing(ctx, 200, "tok-1")
	if l != nil {
		t.Error("expected nil for wrong owner")
	}

	// SaveListings batch
	records := []storage.ListingRecord{
		{Token: "batch-1", ChatID: 100, SearchID: searchID, SearchName: "mazda3", Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 85000},
		{Token: "batch-2", ChatID: 100, SearchID: searchID, SearchName: "mazda3", Manufacturer: "Honda", Model: "Civic", Year: 2022, Price: 100000},
	}
	if err := store.SaveListings(ctx, records); err != nil {
		t.Fatalf("batch save: %v", err)
	}

	// Empty batch should be no-op
	if err := store.SaveListings(ctx, nil); err != nil {
		t.Fatalf("empty batch: %v", err)
	}

	// CountUserListings
	count, err := store.CountUserListings(ctx, 100)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}

	// ListUserListings pagination
	page1, _ := store.ListUserListings(ctx, 100, 2, 0)
	if len(page1) != 2 {
		t.Errorf("page 1: expected 2, got %d", len(page1))
	}
	page2, _ := store.ListUserListings(ctx, 100, 2, 2)
	if len(page2) != 1 {
		t.Errorf("page 2: expected 1, got %d", len(page2))
	}

	// ListSearchListings
	listings, err := store.ListSearchListings(ctx, 100, searchID, storage.ListingFilter{}, 20, 0, "newest")
	if err != nil {
		t.Fatalf("ListSearchListings: %v", err)
	}
	if len(listings) != 3 {
		t.Errorf("expected 3, got %d", len(listings))
	}

	// CountSearchListings
	cnt, err := store.CountSearchListings(ctx, 100, searchID, storage.ListingFilter{})
	if err != nil {
		t.Fatalf("CountSearchListings: %v", err)
	}
	if cnt != 3 {
		t.Errorf("expected 3, got %d", cnt)
	}

	// Upsert conflict
	if err := store.SaveListings(ctx, []storage.ListingRecord{
		{Token: "batch-1", ChatID: 100, SearchID: searchID, SearchName: "mazda3", Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000, Km: 60000},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	l, _ = store.GetListing(ctx, 100, "batch-1")
	if l.Price != 80000 || l.Km != 60000 {
		t.Errorf("after upsert: price=%d km=%d", l.Price, l.Km)
	}
	// Should still be 3 listings total
	count, _ = store.CountUserListings(ctx, 100)
	if count != 3 {
		t.Errorf("upsert should not create duplicate, got %d", count)
	}
}

func TestPostgres_ListSearchListings_Filters(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	searchID, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "test", Source: "yad2",
		Manufacturer: 8, Model: 10061,
	})

	// lsl-1: Year=2021, Price=120000, Km=50000, Hand=2, Commercial
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "lsl-1", ChatID: 100, SearchID: searchID, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 120000, Km: 50000, Hand: 2,
		IsCommercial: ptrBoolPg(true),
	}); err != nil {
		t.Fatal(err)
	}
	// lsl-2: Year=2022, Price=90000, Km=30000, Hand=1, Private
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "lsl-2", ChatID: 100, SearchID: searchID, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2022, Price: 90000, Km: 30000, Hand: 1,
		IsCommercial: ptrBoolPg(false),
	}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		filter storage.ListingFilter
		want   int
	}{
		{"no filter", storage.ListingFilter{}, 2},
		{"price_max excludes expensive", storage.ListingFilter{PriceMax: 100000}, 1},
		{"year_min", storage.ListingFilter{YearMin: 2022}, 1},
		{"year_max", storage.ListingFilter{YearMax: 2021}, 1},
		{"max_km", storage.ListingFilter{MaxKm: 40000}, 1},
		{"max_hand", storage.ListingFilter{MaxHand: 1}, 1},
		{"combined", storage.ListingFilter{PriceMax: 100000, YearMin: 2022}, 1},
		{"all filtered out", storage.ListingFilter{PriceMax: 50000}, 0},
		{"commercial only", storage.ListingFilter{Commercial: ptrBoolPg(true)}, 1},
		{"private only", storage.ListingFilter{Commercial: ptrBoolPg(false)}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.ListSearchListings(ctx, 100, searchID, tc.filter, 20, 0, "newest")
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("got %d rows, want %d", len(got), tc.want)
			}
			cnt, err := store.CountSearchListings(ctx, 100, searchID, tc.filter)
			if err != nil {
				t.Fatalf("count: %v", err)
			}
			if int(cnt) != tc.want {
				t.Errorf("count = %d, want %d", cnt, tc.want)
			}
		})
	}

	// Sort orders
	t.Run("sort price_asc", func(t *testing.T) {
		listings, _ := store.ListSearchListings(ctx, 100, searchID, storage.ListingFilter{}, 20, 0, "price_asc")
		if listings[0].Price != 90000 {
			t.Errorf("expected 90000 first, got %d", listings[0].Price)
		}
	})
	t.Run("sort price_desc", func(t *testing.T) {
		listings, _ := store.ListSearchListings(ctx, 100, searchID, storage.ListingFilter{}, 20, 0, "price_desc")
		if listings[0].Price != 120000 {
			t.Errorf("expected 120000 first, got %d", listings[0].Price)
		}
	})
	t.Run("sort year", func(t *testing.T) {
		listings, _ := store.ListSearchListings(ctx, 100, searchID, storage.ListingFilter{}, 20, 0, "year")
		if listings[0].Year != 2022 {
			t.Errorf("expected 2022 first, got %d", listings[0].Year)
		}
	})
}

// ---------------------------------------------------------------------------
// Dedup
// ---------------------------------------------------------------------------

func TestPostgres_Dedup(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	// First claim succeeds
	isNew, err := store.ClaimNew(ctx, "token1", 100, 1)
	if err != nil || !isNew {
		t.Fatalf("first claim: new=%v, err=%v", isNew, err)
	}

	// Duplicate claim fails
	isNew, err = store.ClaimNew(ctx, "token1", 100, 1)
	if err != nil || isNew {
		t.Error("duplicate claim for same user should return false")
	}

	// Different user can claim same token
	isNew, err = store.ClaimNew(ctx, "token1", 200, 1)
	if err != nil || !isNew {
		t.Error("same token for different user should be new")
	}

	// ReleaseClaim allows reclaim
	if err := store.ReleaseClaim(ctx, "token1", 100); err != nil {
		t.Fatalf("release: %v", err)
	}
	isNew, _ = store.ClaimNew(ctx, "token1", 100, 1)
	if !isNew {
		t.Error("released token should be claimable again")
	}

	// User 200's claim should be unaffected
	isNew, _ = store.ClaimNew(ctx, "token1", 200, 1)
	if isNew {
		t.Error("user 200's claim should be unaffected by user 100's release")
	}

	// Prune
	_, _ = store.ClaimNew(ctx, "prune-tok", 100, 1)
	pruned, err := store.Prune(ctx, 0)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned < 1 {
		t.Errorf("expected at least 1 pruned, got %d", pruned)
	}

	// Prune keeps recent
	_, _ = store.ClaimNew(ctx, "recent-tok", 100, 1)
	pruned, err = store.Prune(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("prune recent: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned (recent), got %d", pruned)
	}
}

// ---------------------------------------------------------------------------
// Bookmarks
// ---------------------------------------------------------------------------

func TestPostgres_Bookmarks(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	// Save listing first (needed for ListSaved join)
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok1", ChatID: 100, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 95000,
	}); err != nil {
		t.Fatal(err)
	}

	// SaveBookmark
	if err := store.SaveBookmark(ctx, 100, "tok1"); err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}
	count, _ := store.CountSaved(ctx, 100)
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	// Duplicate bookmark is idempotent
	if err := store.SaveBookmark(ctx, 100, "tok1"); err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	count, _ = store.CountSaved(ctx, 100)
	if count != 1 {
		t.Errorf("duplicate should not increase count, got %d", count)
	}

	// IsSaved
	saved, _ := store.IsSaved(ctx, 100, "tok1")
	if !saved {
		t.Error("tok1 should be saved")
	}
	saved, _ = store.IsSaved(ctx, 100, "nonexistent")
	if saved {
		t.Error("nonexistent should not be saved")
	}

	// ListSaved
	listings, err := store.ListSaved(ctx, 100, 10, 0)
	if err != nil {
		t.Fatalf("ListSaved: %v", err)
	}
	if len(listings) != 1 || listings[0].Manufacturer != "Mazda" {
		t.Errorf("ListSaved = %+v", listings)
	}

	// Cross-user isolation
	_ = store.SaveBookmark(ctx, 200, "tok2")
	count1, _ := store.CountSaved(ctx, 100)
	count2, _ := store.CountSaved(ctx, 200)
	if count1 != 1 || count2 != 1 {
		t.Errorf("counts: user100=%d user200=%d", count1, count2)
	}

	// SavedAmong
	_ = store.SaveBookmark(ctx, 100, "tok-a")
	m, err := store.SavedAmong(ctx, 100, []string{"tok1", "tok-a", "tok-missing"})
	if err != nil {
		t.Fatalf("SavedAmong: %v", err)
	}
	if !m["tok1"] || !m["tok-a"] || m["tok-missing"] {
		t.Errorf("SavedAmong = %v", m)
	}

	// RemoveBookmark
	if err := store.RemoveBookmark(ctx, 100, "tok1"); err != nil {
		t.Fatalf("RemoveBookmark: %v", err)
	}
	count, _ = store.CountSaved(ctx, 100)
	if count != 1 { // tok-a remains
		t.Errorf("after remove: count = %d, want 1", count)
	}

	// RemoveBookmark nonexistent is silent
	if err := store.RemoveBookmark(ctx, 100, "nonexistent"); err != nil {
		t.Fatalf("remove nonexistent: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Hidden
// ---------------------------------------------------------------------------

func TestPostgres_Hidden(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	// HideListing
	if err := store.HideListing(ctx, 100, "tok1"); err != nil {
		t.Fatalf("HideListing: %v", err)
	}
	hidden, _ := store.IsHidden(ctx, 100, "tok1")
	if !hidden {
		t.Error("tok1 should be hidden")
	}

	// Duplicate is idempotent
	if err := store.HideListing(ctx, 100, "tok1"); err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	count, _ := store.CountHidden(ctx, 100)
	if count != 1 {
		t.Errorf("duplicate should not increase count, got %d", count)
	}

	// Cross-user isolation
	hidden200, _ := store.IsHidden(ctx, 200, "tok1")
	if hidden200 {
		t.Error("user 200 should NOT see tok1 as hidden")
	}

	// ListHiddenTokens
	_ = store.HideListing(ctx, 100, "tok2")
	tokens, err := store.ListHiddenTokens(ctx, 100)
	if err != nil {
		t.Fatalf("ListHiddenTokens: %v", err)
	}
	if len(tokens) != 2 || !tokens["tok1"] || !tokens["tok2"] {
		t.Errorf("tokens = %v", tokens)
	}

	// CountHidden cross-user
	_ = store.HideListing(ctx, 200, "tok3")
	c1, _ := store.CountHidden(ctx, 100)
	c2, _ := store.CountHidden(ctx, 200)
	if c1 != 2 || c2 != 1 {
		t.Errorf("counts: user100=%d user200=%d", c1, c2)
	}

	// ListHidden pagination
	for i := 0; i < 3; i++ {
		_ = store.HideListing(ctx, 100, "extra-"+string(rune('a'+i)))
	}
	page1, _ := store.ListHidden(ctx, 100, 2, 0)
	if len(page1) != 2 {
		t.Errorf("page 1: expected 2, got %d", len(page1))
	}

	// UnhideListing
	if err := store.UnhideListing(ctx, 100, "tok1"); err != nil {
		t.Fatalf("UnhideListing: %v", err)
	}
	hidden, _ = store.IsHidden(ctx, 100, "tok1")
	if hidden {
		t.Error("tok1 should no longer be hidden")
	}
	// tok2 still hidden
	hidden, _ = store.IsHidden(ctx, 100, "tok2")
	if !hidden {
		t.Error("tok2 should still be hidden")
	}

	// UnhideListing nonexistent is silent
	if err := store.UnhideListing(ctx, 100, "nonexistent"); err != nil {
		t.Fatalf("unhide nonexistent: %v", err)
	}

	// ClearHidden
	if err := store.ClearHidden(ctx, 100); err != nil {
		t.Fatalf("ClearHidden: %v", err)
	}
	c1, _ = store.CountHidden(ctx, 100)
	c2, _ = store.CountHidden(ctx, 200)
	if c1 != 0 {
		t.Errorf("user 100 should have 0 hidden after clear, got %d", c1)
	}
	if c2 != 1 {
		t.Errorf("user 200 should still have 1, got %d", c2)
	}
}

// ---------------------------------------------------------------------------
// Price tracking
// ---------------------------------------------------------------------------

func TestPostgres_PriceTracking(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// First record: not a change
	_, changed, err := store.RecordPrice(ctx, "token1", 100000)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("first price should not be a change")
	}

	// Price drop
	oldPrice, changed, err := store.RecordPrice(ctx, "token1", 90000)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("price drop should be detected")
	}
	if oldPrice != 100000 {
		t.Errorf("old price = %d, want 100000", oldPrice)
	}

	// Price increase
	oldPrice, changed, err = store.RecordPrice(ctx, "token1", 95000)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("price increase should be detected")
	}
	if oldPrice != 90000 {
		t.Errorf("old price = %d, want 90000", oldPrice)
	}

	// Same price is not a change
	oldPrice, changed, err = store.RecordPrice(ctx, "token1", 95000)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("same price should not be a change")
	}
	if oldPrice != 95000 {
		t.Errorf("old price = %d, want 95000", oldPrice)
	}

	// GetPriceHistory
	points, err := store.GetPriceHistory(ctx, "token1")
	if err != nil {
		t.Fatalf("GetPriceHistory: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 price points, got %d", len(points))
	}
	// Most recent first
	if points[0].Price != 95000 {
		t.Errorf("most recent = %d, want 95000", points[0].Price)
	}
	if points[2].Price != 100000 {
		t.Errorf("oldest = %d, want 100000", points[2].Price)
	}

	// GetPriceHistory empty
	points, _ = store.GetPriceHistory(ctx, "nonexistent")
	if len(points) != 0 {
		t.Errorf("expected 0, got %d", len(points))
	}

	// PrunePrices keeps recent
	pruned, _ := store.PrunePrices(ctx, 24*time.Hour)
	if pruned != 0 {
		t.Errorf("expected 0 pruned (recent), got %d", pruned)
	}

	// PrunePrices removes all with zero duration
	pruned, err = store.PrunePrices(ctx, 0)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 3 {
		t.Errorf("expected 3 pruned, got %d", pruned)
	}
}

// ---------------------------------------------------------------------------
// Market medians (materialized view)
// ---------------------------------------------------------------------------

func TestPostgres_MarketMedians(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	now := time.Now().UTC()

	// Seed enough listings for a cohort (>= 10 with price > 5000).
	for i := range 12 {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: fmt.Sprintf("mkt-%d", i), ChatID: 100, SearchName: "toyota",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020,
			Price: 90000 + i*2000, Km: 40000 + i*1000,
			FirstSeenAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// A few listings with empty manufacturer (should be excluded).
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "mkt-empty", ChatID: 100, SearchName: "test",
		Manufacturer: "", Model: "X", Year: 2020,
		Price: 50000, FirstSeenAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	// A few listings below the price floor (should be excluded).
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "mkt-cheap", ChatID: 100, SearchName: "test",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020,
		Price: 1000, FirstSeenAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	// Refresh the materialized view.
	if err := store.RefreshMarketMedians(ctx); err != nil {
		t.Fatalf("RefreshMarketMedians: %v", err)
	}

	// Load the computed medians.
	rows, err := store.LoadMarketMedians(ctx)
	if err != nil {
		t.Fatalf("LoadMarketMedians: %v", err)
	}

	// Should have exactly one cohort (Toyota Corolla 2020).
	if len(rows) != 1 {
		t.Fatalf("expected 1 cohort, got %d", len(rows))
	}

	r := rows[0]
	if r.Manufacturer != "Toyota" || r.Model != "Corolla" || r.Year != 2020 {
		t.Errorf("unexpected cohort: %s %s %d", r.Manufacturer, r.Model, r.Year)
	}
	if r.CohortSize != 12 {
		t.Errorf("expected cohort_size=12, got %d", r.CohortSize)
	}
	// Median of 90000..112000 (12 values): avg of 6th and 7th = (100000+102000)/2 = 101000
	if r.MedianPrice < 95000 || r.MedianPrice > 107000 {
		t.Errorf("median_price=%d out of expected range [95000, 107000]", r.MedianPrice)
	}
	if r.MedianKm <= 0 {
		t.Error("median_km should be positive")
	}
}

// ---------------------------------------------------------------------------
// Link tokens
// ---------------------------------------------------------------------------

func TestPostgres_LinkTokens(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	// Create web user for linking
	webID, err := store.UpsertWebUser(ctx, "firebase-uid-1", "test@example.com")
	if err != nil {
		t.Fatalf("create web user: %v", err)
	}

	// CreateLinkToken
	token, err := store.CreateLinkToken(ctx, webID)
	if err != nil {
		t.Fatalf("CreateLinkToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// ConsumeLinkToken success
	gotID, err := store.ConsumeLinkToken(ctx, token)
	if err != nil {
		t.Fatalf("ConsumeLinkToken: %v", err)
	}
	if gotID != webID {
		t.Errorf("got webChatID %d, want %d", gotID, webID)
	}

	// ConsumeLinkToken already used
	_, err = store.ConsumeLinkToken(ctx, token)
	if !errors.Is(err, storage.ErrLinkTokenUsed) {
		t.Errorf("expected ErrLinkTokenUsed, got %v", err)
	}

	// ConsumeLinkToken not found
	_, err = store.ConsumeLinkToken(ctx, "nonexistent-token")
	if !errors.Is(err, storage.ErrLinkTokenNotFound) {
		t.Errorf("expected ErrLinkTokenNotFound, got %v", err)
	}

	// Expired token: insert with past expiry directly
	expiredToken := "expired-test-token"
	_, _ = store.DB().ExecContext(ctx, `
		INSERT INTO link_tokens (token, web_chat_id, expires_at, used)
		VALUES ($1, $2, $3, FALSE)`,
		expiredToken, webID, time.Now().UTC().Add(-1*time.Hour))
	_, err = store.ConsumeLinkToken(ctx, expiredToken)
	if !errors.Is(err, storage.ErrLinkTokenExpired) {
		t.Errorf("expected ErrLinkTokenExpired, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete search cascade
// ---------------------------------------------------------------------------

func TestPostgres_DeleteSearchCascade(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	id1, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "mazda-3", Manufacturer: 27, Model: 10332,
	})
	id2, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 200, Name: "mazda-3", Manufacturer: 27, Model: 10332,
	})

	// Seed seen_listings
	_, _ = store.ClaimNew(ctx, "tok1", 100, id1)
	_, _ = store.ClaimNew(ctx, "tok1", 200, id2)

	// Seed listing_history
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok1", ChatID: 100, SearchID: id1, SearchName: "mazda-3",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 95000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok1", ChatID: 200, SearchID: id2, SearchName: "mazda-3",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 95000,
	}); err != nil {
		t.Fatal(err)
	}

	// Delete user 100's search
	if err := store.DeleteSearch(ctx, id1, 100); err != nil {
		t.Fatalf("delete search: %v", err)
	}

	// Search gone
	s, _ := store.GetSearch(ctx, id1, 100)
	if s != nil {
		t.Error("search should be deleted")
	}

	// seen_listings for user 100 cleaned up
	isNew, _ := store.ClaimNew(ctx, "tok1", 100, id1)
	if !isNew {
		t.Error("tok1 claim for user 100 should be released")
	}

	// User 200's claim untouched
	isNew, _ = store.ClaimNew(ctx, "tok1", 200, id2)
	if isNew {
		t.Error("user 200's claim should NOT be affected")
	}

	// listing_history for user 100 cleaned up
	count100, _ := store.CountSearchListings(ctx, 100, id1, storage.ListingFilter{})
	if count100 != 0 {
		t.Errorf("expected 0 listings for user 100, got %d", count100)
	}

	// User 200's listing_history untouched
	count200, _ := store.CountSearchListings(ctx, 200, id2, storage.ListingFilter{})
	if count200 != 1 {
		t.Errorf("expected 1 listing for user 200, got %d", count200)
	}
}

// ---------------------------------------------------------------------------
// Prune & delete stale listings
// ---------------------------------------------------------------------------

func TestPostgres_PruneListings(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	old := time.Now().Add(-100 * 24 * time.Hour)
	recent := time.Now().Add(-10 * 24 * time.Hour)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "old-1", ChatID: 100, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 90000, FirstSeenAt: old,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "recent-1", ChatID: 100, SearchName: "test",
		Manufacturer: "Honda", Model: "Civic", Year: 2022, Price: 95000, FirstSeenAt: recent,
	}); err != nil {
		t.Fatal(err)
	}

	pruned, err := store.PruneListings(ctx, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("PruneListings: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	count, _ := store.CountUserListings(ctx, 100)
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestPostgres_PruneListingsPreservesSaved(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	old := time.Now().Add(-100 * 24 * time.Hour)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "old-saved", ChatID: 100, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 90000, FirstSeenAt: old,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "old-unsaved", ChatID: 100, SearchName: "test",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 85000, FirstSeenAt: old,
	}); err != nil {
		t.Fatal(err)
	}
	_ = store.SaveBookmark(ctx, 100, "old-saved")

	pruned, _ := store.PruneListings(ctx, 90*24*time.Hour)
	if pruned != 1 {
		t.Errorf("expected 1 pruned (unsaved only), got %d", pruned)
	}

	saved, _ := store.GetListing(ctx, 100, "old-saved")
	if saved == nil {
		t.Error("saved listing should survive pruning")
	}
}

func TestPostgres_DeleteStaleListings(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	searchID, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "test", Manufacturer: 1, Model: 1,
	})

	for _, tok := range []string{"a", "b", "c", "d"} {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: tok, ChatID: 100, SearchID: searchID, SearchName: "test",
			Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 90000,
		}); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := store.DeleteStaleListings(ctx, 100, searchID, []string{"a", "c"})
	if err != nil {
		t.Fatalf("DeleteStaleListings: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	// Kept tokens still exist
	for _, tok := range []string{"a", "c"} {
		l, _ := store.GetListing(ctx, 100, tok)
		if l == nil {
			t.Errorf("listing %q should still exist", tok)
		}
	}
	// Removed tokens gone
	for _, tok := range []string{"b", "d"} {
		l, _ := store.GetListing(ctx, 100, tok)
		if l != nil {
			t.Errorf("listing %q should be deleted", tok)
		}
	}
}

// ---------------------------------------------------------------------------
// PriceList
// ---------------------------------------------------------------------------

func TestPostgres_PriceList(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// Not found
	e, err := store.GetPriceListEntry(ctx, 12345, 2022)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if e != nil {
		t.Error("expected nil for nonexistent entry")
	}

	// Set
	if err := store.SetPriceListEntry(ctx, storage.PriceListEntry{
		SubModelID: 12345, Year: 2022, BasePrice: 150000, Title: "Mazda 3 Sedan",
	}); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Get
	e, err = store.GetPriceListEntry(ctx, 12345, 2022)
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil entry")
	}
	if e.BasePrice != 150000 || e.Title != "Mazda 3 Sedan" {
		t.Errorf("entry = %+v", e)
	}

	// Upsert updates
	if err := store.SetPriceListEntry(ctx, storage.PriceListEntry{
		SubModelID: 12345, Year: 2022, BasePrice: 145000, Title: "Mazda 3 Sedan",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	e, _ = store.GetPriceListEntry(ctx, 12345, 2022)
	if e.BasePrice != 145000 {
		t.Errorf("after upsert: base_price = %d, want 145000", e.BasePrice)
	}
}

// ---------------------------------------------------------------------------
// Notification center (NewListingsSince, MarkListingUserSeen)
// ---------------------------------------------------------------------------

func TestPostgres_NotificationCenter(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "notif-test", Manufacturer: 1, Model: 1,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "new-1", ChatID: 100, SearchID: searchID, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "new-2", ChatID: 100, SearchID: searchID, SearchName: "s1",
		Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 90000,
		FirstSeenAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	// NewListingsSince
	listings, err := store.NewListingsSince(ctx, 100, cutoff, 20, 0)
	if err != nil {
		t.Fatalf("NewListingsSince: %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("expected 2, got %d", len(listings))
	}

	// CountNewListingsSince
	count, _ := store.CountNewListingsSince(ctx, 100, cutoff)
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// MarkListingUserSeen excludes from results
	if err := store.MarkListingUserSeen(ctx, 100, "new-1"); err != nil {
		t.Fatal(err)
	}
	listings, _ = store.NewListingsSince(ctx, 100, cutoff, 20, 0)
	if len(listings) != 1 {
		t.Errorf("after mark seen: expected 1, got %d", len(listings))
	}
	count, _ = store.CountNewListingsSince(ctx, 100, cutoff)
	if count != 1 {
		t.Errorf("after mark seen: count = %d, want 1", count)
	}

	// UnmarkListingUserSeen brings it back
	if err := store.UnmarkListingUserSeen(ctx, 100, "new-1"); err != nil {
		t.Fatal(err)
	}
	listings, _ = store.NewListingsSince(ctx, 100, cutoff, 20, 0)
	if len(listings) != 2 {
		t.Errorf("after unmark: expected 2, got %d", len(listings))
	}

	// Wrong user sees nothing
	listings, _ = store.NewListingsSince(ctx, 200, cutoff, 20, 0)
	if len(listings) != 0 {
		t.Errorf("wrong user: expected 0, got %d", len(listings))
	}

	// Future cutoff returns nothing
	listings, _ = store.NewListingsSince(ctx, 100, time.Now().Add(time.Hour), 20, 0)
	if len(listings) != 0 {
		t.Errorf("future cutoff: expected 0, got %d", len(listings))
	}
}

// ---------------------------------------------------------------------------
// WhatsApp / Web user channels
// ---------------------------------------------------------------------------

func TestPostgres_ChannelUsers(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// WhatsApp user
	waID1, err := store.UpsertWhatsAppUser(ctx, "+972501234567")
	if err != nil {
		t.Fatalf("create whatsapp: %v", err)
	}
	if waID1 < 1_000_000_000_000 {
		t.Errorf("WhatsApp ID = %d, want >= 1T", waID1)
	}

	// Idempotent
	waID2, _ := store.UpsertWhatsAppUser(ctx, "+972501234567")
	if waID2 != waID1 {
		t.Errorf("idempotent: %d vs %d", waID2, waID1)
	}

	// Different phone
	waID3, _ := store.UpsertWhatsAppUser(ctx, "+972509876543")
	if waID3 == waID1 {
		t.Error("different phone numbers should get different IDs")
	}

	// Channel fields
	u, _ := store.GetUser(ctx, waID1)
	if u.Channel != "whatsapp" || u.ChannelID != "+972501234567" {
		t.Errorf("channel=%q channelID=%q", u.Channel, u.ChannelID)
	}

	// Web user
	webID1, err := store.UpsertWebUser(ctx, "firebase-uid-1", "a@example.com")
	if err != nil {
		t.Fatalf("create web: %v", err)
	}
	if webID1 < 2_000_000_000_000 {
		t.Errorf("Web ID = %d, want >= 2T", webID1)
	}

	webU, _ := store.GetUser(ctx, webID1)
	if webU.Channel != "web" || webU.ChannelID != "firebase-uid-1" || webU.Username != "a@example.com" {
		t.Errorf("web user = %+v", webU)
	}

	// GetUserByChannelID
	found, _ := store.GetUserByChannelID(ctx, "whatsapp", "+972501234567")
	if found == nil || found.ChatID != waID1 {
		t.Errorf("GetUserByChannelID = %+v", found)
	}
	notFound, _ := store.GetUserByChannelID(ctx, "whatsapp", "+000000000")
	if notFound != nil {
		t.Error("expected nil for unknown phone")
	}
}

// ---------------------------------------------------------------------------
// CountSearchListingsForChat
// ---------------------------------------------------------------------------

func TestPostgres_CountSearchListingsForChat(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	idCapped, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "capped", Source: "yad2",
		Manufacturer: 8, Model: 10061, PriceMax: 100000,
	})
	idOpen, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "open", Source: "yad2",
		Manufacturer: 9, Model: 10062,
	})

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "cfc-1", ChatID: 100, SearchID: idCapped, SearchName: "capped",
		Manufacturer: "Mazda", Model: "3", Year: 2022, Price: 90000, Km: 30000, Hand: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "cfc-2", ChatID: 100, SearchID: idCapped, SearchName: "capped",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 120000, Km: 50000, Hand: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "cfc-3", ChatID: 100, SearchID: idOpen, SearchName: "open",
		Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 80000, Km: 70000, Hand: 3,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := store.CountSearchListingsForChat(ctx, 100)
	if err != nil {
		t.Fatalf("CountSearchListingsForChat: %v", err)
	}

	// capped search has PriceMax=100000, so only cfc-1 (90000) qualifies
	if got[idCapped] != 1 {
		t.Errorf("capped search: got %d, want 1", got[idCapped])
	}
	if got[idOpen] != 1 {
		t.Errorf("open search: got %d, want 1", got[idOpen])
	}
}

// ---------------------------------------------------------------------------
// Admin
// ---------------------------------------------------------------------------

func TestPostgres_Admin(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	// DBSizeBytes
	size, err := store.DBSizeBytes()
	if err != nil {
		t.Fatalf("DBSizeBytes: %v", err)
	}
	if size <= 0 {
		t.Errorf("expected positive size, got %d", size)
	}

	// CountAllListings (empty)
	cnt, _ := store.CountAllListings(ctx)
	if cnt != 0 {
		t.Errorf("expected 0, got %d", cnt)
	}

	// Insert listings
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "admin-1", ChatID: 100, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
		FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "admin-2", ChatID: 100, SearchName: "s1",
		Manufacturer: "Honda", Model: "Civic", Year: 2021, Price: 90000,
		FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cnt, _ = store.CountAllListings(ctx)
	if cnt != 2 {
		t.Errorf("expected 2, got %d", cnt)
	}

	// TableSizes
	sizes, err := store.TableSizes(ctx)
	if err != nil {
		t.Fatalf("TableSizes: %v", err)
	}
	if sizes["users"] != 1 {
		t.Errorf("users count = %d, want 1", sizes["users"])
	}

	// AdminListListings
	items, total, err := store.AdminListListings(ctx, 1, 0, 0)
	if err != nil {
		t.Fatalf("AdminListListings: %v", err)
	}
	if total != 2 || len(items) != 1 {
		t.Errorf("total=%d items=%d", total, len(items))
	}

	// AdminDeleteListing
	if err := store.AdminDeleteListing(ctx, "admin-1", 100); err != nil {
		t.Fatalf("AdminDeleteListing: %v", err)
	}
	l, _ := store.GetListing(ctx, 100, "admin-1")
	if l != nil {
		t.Error("listing should be deleted")
	}

	// DBPoolStats
	stats := store.DBPoolStats()
	if stats == nil {
		t.Error("expected non-nil pool stats")
	}
}

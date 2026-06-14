package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestPostgres_AdminListSearchesAndUsers(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	if _, err := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "s100", Manufacturer: 1, Model: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSearch(ctx, storage.Search{ChatID: 200, Name: "s200", Manufacturer: 2, Model: 1}); err != nil {
		t.Fatal(err)
	}

	searches, err := store.AdminListSearches(ctx)
	if err != nil {
		t.Fatalf("AdminListSearches: %v", err)
	}
	if len(searches) != 2 {
		t.Errorf("AdminListSearches = %d rows, want 2", len(searches))
	}

	users, err := store.AdminListUsers(ctx)
	if err != nil {
		t.Fatalf("AdminListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("AdminListUsers = %d rows, want 2", len(users))
	}
}

func TestPostgres_AdminDeleteSearch(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	id, err := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "doomed", Manufacturer: 1, Model: 1})
	if err != nil {
		t.Fatalf("CreateSearch: %v", err)
	}

	// AdminDeleteSearch resolves chat_id from the search id and cascades.
	if err := store.AdminDeleteSearch(ctx, id); err != nil {
		t.Fatalf("AdminDeleteSearch: %v", err)
	}
	if got, _ := store.GetSearch(ctx, id, 100); got != nil {
		t.Error("search should be deleted")
	}

	// Deleting a missing search surfaces the lookup error.
	if err := store.AdminDeleteSearch(ctx, 999999); err == nil {
		t.Error("expected error deleting nonexistent search")
	}
}

func TestPostgres_SyncUserActiveStatus(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100) // active, no searches -> deactivated
	seedPgUser(t, store, 200) // has active search but inactive -> activated

	if _, err := store.CreateSearch(ctx, storage.Search{ChatID: 200, Name: "s", Manufacturer: 1, Model: 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserActive(ctx, 200, false); err != nil {
		t.Fatal(err)
	}

	activated, deactivated, err := store.SyncUserActiveStatus(ctx)
	if err != nil {
		t.Fatalf("SyncUserActiveStatus: %v", err)
	}
	if activated != 1 {
		t.Errorf("activated = %d, want 1", activated)
	}
	if deactivated != 1 {
		t.Errorf("deactivated = %d, want 1", deactivated)
	}

	u100, _ := store.GetUser(ctx, 100)
	u200, _ := store.GetUser(ctx, 200)
	if u100.Active {
		t.Error("user 100 (no searches) should be inactive")
	}
	if !u200.Active {
		t.Error("user 200 (active search) should be active")
	}
}

func TestPostgres_AdminActivityStats(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "act-1", ChatID: 100, SearchName: "s",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
		FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	// days=7 spans CURRENT_DATE-7 .. CURRENT_DATE inclusive => 8 buckets.
	items, err := store.AdminActivityStats(ctx, 7)
	if err != nil {
		t.Fatalf("AdminActivityStats: %v", err)
	}
	if len(items) != 8 {
		t.Fatalf("expected 8 day buckets, got %d", len(items))
	}
	// Today is the last bucket and should count the listing saved above.
	if items[len(items)-1].NewListings < 1 {
		t.Errorf("today bucket NewListings = %d, want >= 1", items[len(items)-1].NewListings)
	}

	// days<=0 falls back to a 30-day window (31 buckets).
	if def, _ := store.AdminActivityStats(ctx, 0); len(def) != 31 {
		t.Errorf("default window = %d buckets, want 31", len(def))
	}
}

func TestPostgres_ResetAllData(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if _, err := store.CreateSearch(ctx, storage.Search{ChatID: 100, Name: "s", Manufacturer: 1, Model: 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "reset-1", ChatID: 100, SearchName: "s",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
		FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	counts, err := store.ResetAllData(ctx)
	if err != nil {
		t.Fatalf("ResetAllData: %v", err)
	}
	if counts["listing_history"] != 1 {
		t.Errorf("counts[listing_history] = %d, want 1", counts["listing_history"])
	}

	// Listing data is wiped; users/searches are preserved (not in resetTables).
	if n, _ := store.CountAllListings(ctx); n != 0 {
		t.Errorf("listings after reset = %d, want 0", n)
	}
	if n, _ := store.CountUsers(ctx); n != 1 {
		t.Errorf("users after reset = %d, want 1 (preserved)", n)
	}
}

func TestPostgres_VacuumAndFileSize(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if err := store.VacuumDB(ctx); err != nil {
		t.Fatalf("VacuumDB: %v", err)
	}

	size, err := store.DBFileSize()
	if err != nil {
		t.Fatalf("DBFileSize: %v", err)
	}
	if size <= 0 {
		t.Errorf("DBFileSize = %d, want > 0", size)
	}
}

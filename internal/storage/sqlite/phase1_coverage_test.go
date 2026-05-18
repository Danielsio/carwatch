package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// --- UnmarkListingUserSeen ---

func TestUnmarkListingUserSeen(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	searchID := seedSearchForNotif(t, store, 100)

	cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "unsee-1", ChatID: 100, SearchID: searchID, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.MarkListingUserSeen(ctx, 100, "unsee-1"); err != nil {
		t.Fatal(err)
	}
	listings, err := store.NewListingsSince(ctx, 100, cutoff, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 0 {
		t.Fatalf("after mark seen: expected 0, got %d", len(listings))
	}

	if err = store.UnmarkListingUserSeen(ctx, 100, "unsee-1"); err != nil {
		t.Fatal(err)
	}
	listings, err = store.NewListingsSince(ctx, 100, cutoff, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Errorf("after unmark: expected 1, got %d", len(listings))
	}
}

func TestUnmarkListingUserSeen_EmptyToken(t *testing.T) {
	store := newTestStore(t)
	if err := store.UnmarkListingUserSeen(context.Background(), 100, ""); err != nil {
		t.Errorf("empty token should be no-op, got %v", err)
	}
}

func TestMarkListingUserSeen_EmptyToken(t *testing.T) {
	store := newTestStore(t)
	if err := store.MarkListingUserSeen(context.Background(), 100, ""); err != nil {
		t.Errorf("empty token should be no-op, got %v", err)
	}
}

// --- ListingUserSeenAmong ---

func TestListingUserSeenAmong(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.MarkListingUserSeen(ctx, 100, "tok-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkListingUserSeen(ctx, 100, "tok-b"); err != nil {
		t.Fatal(err)
	}

	result, err := store.ListingUserSeenAmong(ctx, 100, []string{"tok-a", "tok-b", "tok-c"})
	if err != nil {
		t.Fatal(err)
	}
	if !result["tok-a"] || !result["tok-b"] {
		t.Error("tok-a and tok-b should be seen")
	}
	if result["tok-c"] {
		t.Error("tok-c should not be seen")
	}
}

func TestListingUserSeenAmong_EmptyInput(t *testing.T) {
	store := newTestStore(t)
	result, err := store.ListingUserSeenAmong(context.Background(), 100, nil)
	if err != nil || result != nil {
		t.Errorf("nil input: expected nil,nil got %v,%v", result, err)
	}
	result, err = store.ListingUserSeenAmong(context.Background(), 100, []string{})
	if err != nil || result != nil {
		t.Errorf("empty input: expected nil,nil got %v,%v", result, err)
	}
}

func TestListingUserSeenAmong_DeduplicatesAndSkipsEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.MarkListingUserSeen(ctx, 100, "dup-tok"); err != nil {
		t.Fatal(err)
	}

	result, err := store.ListingUserSeenAmong(ctx, 100, []string{"dup-tok", "dup-tok", "", "dup-tok"})
	if err != nil {
		t.Fatal(err)
	}
	if !result["dup-tok"] {
		t.Error("dup-tok should be seen")
	}
}

// --- PriceListEntry ---

func TestPriceListEntry_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	entry := storage.PriceListEntry{
		SubModelID: 12345,
		Year:       2022,
		BasePrice:  150000,
		Title:      "Corolla GLi",
	}
	if err := store.SetPriceListEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetPriceListEntry(ctx, 12345, 2022)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected entry, got nil")
	}
	if got.BasePrice != 150000 || got.Title != "Corolla GLi" {
		t.Errorf("unexpected entry: %+v", got)
	}
}

func TestPriceListEntry_NotFound(t *testing.T) {
	store := newTestStore(t)
	got, err := store.GetPriceListEntry(context.Background(), 99999, 2030)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent entry, got %+v", got)
	}
}

func TestPriceListEntry_Upsert(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.SetPriceListEntry(ctx, storage.PriceListEntry{
		SubModelID: 100, Year: 2020, BasePrice: 80000, Title: "Old",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPriceListEntry(ctx, storage.PriceListEntry{
		SubModelID: 100, Year: 2020, BasePrice: 90000, Title: "Updated",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetPriceListEntry(ctx, 100, 2020)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected entry after upsert, got nil")
	}
	if got.BasePrice != 90000 || got.Title != "Updated" {
		t.Errorf("upsert did not update: %+v", got)
	}
}

// --- BackfillListings ---

func TestBackfillListings_FillsMissingFieldsOnly(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "bf-1", ChatID: 100, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		Km: 0, City: "", ImageURL: "",
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "bf-2", ChatID: 100, SearchName: "s1",
		Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 90000,
		Km: 50000, City: "Haifa", ImageURL: "https://img.com/1.jpg",
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.BackfillListings(ctx, []storage.ListingRecord{
		{Token: "bf-1", Km: 30000, City: "Tel Aviv", ImageURL: "https://img.com/2.jpg"},
		{Token: "bf-2", Km: 99999, City: "Eilat", ImageURL: "https://img.com/3.jpg"},
	}); err != nil {
		t.Fatal(err)
	}

	l1, _ := store.GetListing(ctx, 100, "bf-1")
	if l1.Km != 30000 || l1.City != "Tel Aviv" || l1.ImageURL != "https://img.com/2.jpg" {
		t.Errorf("bf-1 should be backfilled: km=%d city=%s img=%s", l1.Km, l1.City, l1.ImageURL)
	}

	l2, _ := store.GetListing(ctx, 100, "bf-2")
	if l2.Km != 50000 || l2.City != "Haifa" || l2.ImageURL != "https://img.com/1.jpg" {
		t.Errorf("bf-2 should NOT be overwritten: km=%d city=%s img=%s", l2.Km, l2.City, l2.ImageURL)
	}
}

func TestBackfillListings_EmptySlice(t *testing.T) {
	store := newTestStore(t)
	if err := store.BackfillListings(context.Background(), nil); err != nil {
		t.Errorf("empty backfill should be no-op, got %v", err)
	}
}

// --- PruneLinkTokens ---

func TestPruneLinkTokens(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	fresh, err := store.CreateLinkToken(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}

	expired, err := generateLinkTokenBytes()
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.db.ExecContext(ctx,
		`INSERT INTO link_tokens (token, web_chat_id, expires_at, used)
		 VALUES (?, ?, ?, 0)`,
		expired, int64(2000), time.Now().UTC().Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	pruned, err := store.PruneLinkTokens(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	_, err = store.ConsumeLinkToken(ctx, fresh)
	if err != nil {
		t.Errorf("fresh token should still be valid, got %v", err)
	}
}

// --- AdminDeleteUser ---

func TestAdminDeleteUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	if _, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "del-test", Manufacturer: 1, Model: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "del-l1", ChatID: 100, SearchName: "del-test",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.AdminDeleteUser(ctx, 100); err != nil {
		t.Fatal(err)
	}

	searches, err := store.ListSearches(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(searches) != 0 {
		t.Errorf("expected 0 searches after delete, got %d", len(searches))
	}
	listings, err := store.ListUserListings(ctx, 100, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 0 {
		t.Errorf("expected 0 listings after delete, got %d", len(listings))
	}
}

// --- AdminListPriceHistory ---

func TestAdminListPriceHistory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.db.ExecContext(ctx,
		"INSERT INTO price_history (token, price) VALUES (?, ?)", "ph-tok", 100000)
	if err != nil {
		t.Fatal(err)
	}

	items, total, err := store.AdminListPriceHistory(ctx, 20, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Errorf("all: total=%d items=%d", total, len(items))
	}

	items, total, err = store.AdminListPriceHistory(ctx, 20, 0, "ph-tok")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Token != "ph-tok" {
		t.Errorf("filtered: total=%d items=%d", total, len(items))
	}

	items, total, err = store.AdminListPriceHistory(ctx, 20, 0, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("no match: total=%d items=%d", total, len(items))
	}
}

// --- AdminListSeenListings ---

func TestAdminListSeenListings(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "seen-test", Manufacturer: 1, Model: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.ClaimNew(ctx, "seen-tok", 100, searchID); err != nil {
		t.Fatal(err)
	}

	items, total, err := store.AdminListSeenListings(ctx, 20, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total < 1 || len(items) < 1 {
		t.Errorf("all: total=%d items=%d", total, len(items))
	}

	items, total, err = store.AdminListSeenListings(ctx, 20, 0, searchID)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || items[0].Token != "seen-tok" {
		t.Errorf("filtered: total=%d", total)
	}
}

// --- AdminActivityStats ---

func TestAdminActivityStats(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	items, err := store.AdminActivityStats(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 7 {
		t.Errorf("expected at least 7 days, got %d", len(items))
	}
}

func TestAdminActivityStats_DefaultDays(t *testing.T) {
	store := newTestStore(t)
	items, err := store.AdminActivityStats(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 30 {
		t.Errorf("days=0 should default to 30, got %d rows", len(items))
	}
}

// --- DBPoolStats ---

func TestDBPoolStats(t *testing.T) {
	store := newTestStore(t)
	stats := store.DBPoolStats()
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.MaxOpenConnections <= 0 {
		t.Errorf("MaxOpenConnections = %d, want > 0", stats.MaxOpenConnections)
	}
}

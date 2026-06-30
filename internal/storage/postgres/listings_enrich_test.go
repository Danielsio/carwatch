package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestPostgres_BackfillListings(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	// Empty input is a no-op.
	if err := store.BackfillListings(ctx, nil); err != nil {
		t.Fatalf("empty BackfillListings: %v", err)
	}

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "bf-1", ChatID: 100, SearchName: "s",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
		Km: 0, City: "", ImageURL: "", FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	// Backfill fills the empty km/city/image fields.
	if err := store.BackfillListings(ctx, []storage.ListingRecord{
		{Token: "bf-1", Km: 55000, City: "Haifa", ImageURL: "https://img/1"},
	}); err != nil {
		t.Fatalf("BackfillListings: %v", err)
	}
	l, _ := store.GetListing(ctx, 100, "bf-1")
	if l == nil || l.Km != 55000 || l.City != "Haifa" || l.ImageURL != "https://img/1" {
		t.Fatalf("after backfill = %+v", l)
	}

	// A second backfill must NOT overwrite already-populated fields.
	if err := store.BackfillListings(ctx, []storage.ListingRecord{
		{Token: "bf-1", Km: 11111, City: "Eilat", ImageURL: "https://img/2"},
	}); err != nil {
		t.Fatalf("second BackfillListings: %v", err)
	}
	l, _ = store.GetListing(ctx, 100, "bf-1")
	if l.Km != 55000 || l.City != "Haifa" || l.ImageURL != "https://img/1" {
		t.Errorf("backfill overwrote populated fields: %+v", l)
	}
}

func TestPostgres_LookupEnrichmentData(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	// Empty token slice returns nil.
	if m, err := store.LookupEnrichmentData(ctx, nil); err != nil || m != nil {
		t.Fatalf("empty lookup = %v, %v", m, err)
	}

	// en-1 has enrichment data; en-2 has none and must be skipped.
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "en-1", ChatID: 100, SearchName: "search-a",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
		Km: 50000, City: "Tel Aviv", ImageURL: "https://img/en1", BodyType: "sedan", FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "en-2", ChatID: 100, SearchName: "search-b",
		Manufacturer: "Honda", Model: "Civic", Year: 2019, Price: 70000,
		Km: 0, City: "", ImageURL: "", FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	// en-3 is enriched for km/city/image but predates body_type — a legacy row
	// with a NULL body_type. It must still be returned, with BodyType == ""
	// (exercising the COALESCE(body_type, '') in LookupEnrichmentData).
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "en-3", ChatID: 100, SearchName: "search-c",
		Manufacturer: "Mazda", Model: "3", Year: 2021, Price: 90000,
		Km: 60000, City: "Haifa", ImageURL: "https://img/en3", FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx,
		`UPDATE listing_history SET body_type = NULL WHERE token = $1 AND chat_id = $2`, "en-3", int64(100)); err != nil {
		t.Fatalf("null out body_type: %v", err)
	}

	m, err := store.LookupEnrichmentData(ctx, []string{"en-1", "en-2", "en-3", "missing"})
	if err != nil {
		t.Fatalf("LookupEnrichmentData: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 enriched tokens, got %d (%+v)", len(m), m)
	}
	rec, ok := m["en-1"]
	if !ok {
		t.Fatal("en-1 missing from result")
	}
	if rec.Km != 50000 || rec.City != "Tel Aviv" || rec.ImageURL != "https://img/en1" || rec.SearchName != "search-a" {
		t.Errorf("enrichment record = %+v", rec)
	}
	if rec.BodyType != "sedan" {
		t.Errorf("enrichment record body_type = %q, want sedan", rec.BodyType)
	}

	legacy, ok := m["en-3"]
	if !ok {
		t.Fatal("en-3 (NULL body_type) missing from result")
	}
	if legacy.Km != 60000 || legacy.City != "Haifa" {
		t.Errorf("legacy enrichment record = %+v", legacy)
	}
	if legacy.BodyType != "" {
		t.Errorf("legacy body_type = %q, want \"\" (NULL coalesced)", legacy.BodyType)
	}
}

func TestPostgres_LookupListingIdentity(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "id-1", ChatID: 100, SearchName: "search-a",
		Manufacturer: "Mazda", Model: "3", Year: 2022, Price: 95000,
		FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	id, err := store.LookupListingIdentity(ctx, "id-1")
	if err != nil {
		t.Fatalf("LookupListingIdentity: %v", err)
	}
	if id.Manufacturer != "Mazda" || id.Model != "3" || id.Year != 2022 || id.Price != 95000 || id.SearchName != "search-a" {
		t.Errorf("identity = %+v", id)
	}

	if _, err := store.LookupListingIdentity(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("missing token = %v, want ErrNotFound", err)
	}
}

func TestPostgres_IncrementEnrichAttempt(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "ia-1", ChatID: 100, SearchName: "s",
		Manufacturer: "Kia", Model: "Niro", Year: 2021, Price: 100000,
		FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.IncrementEnrichAttempt(ctx, "ia-1"); err != nil {
		t.Fatalf("IncrementEnrichAttempt: %v", err)
	}
	if err := store.IncrementEnrichAttempt(ctx, "ia-1"); err != nil {
		t.Fatalf("IncrementEnrichAttempt 2: %v", err)
	}

	var attempts int
	var lastEnrich time.Time
	if err := store.DB().QueryRowContext(ctx,
		`SELECT enrich_attempts, last_enrich_at FROM listing_history WHERE token = $1`, "ia-1").
		Scan(&attempts, &lastEnrich); err != nil {
		t.Fatalf("read enrich columns: %v", err)
	}
	if attempts != 2 {
		t.Errorf("enrich_attempts = %d, want 2", attempts)
	}
	if lastEnrich.IsZero() {
		t.Error("last_enrich_at should be stamped")
	}
}

func TestPostgres_ListUserListingsPagination(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	const total = 5
	base := time.Now().UTC()
	for i := 0; i < total; i++ {
		// Distinct, strictly-decreasing first_seen_at so DESC order is stable.
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: fmt.Sprintf("pg-%d", i), ChatID: 100, SearchName: "s",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
			FirstSeenAt: base.Add(-time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// Walk pages of 2 and ensure every token is returned exactly once.
	seen := map[string]bool{}
	for offset := 0; offset < total; offset += 2 {
		page, err := store.ListUserListings(ctx, 100, 2, offset)
		if err != nil {
			t.Fatalf("page offset=%d: %v", offset, err)
		}
		want := 2
		if offset+2 > total {
			want = total - offset
		}
		if len(page) != want {
			t.Errorf("offset=%d page size = %d, want %d", offset, len(page), want)
		}
		for _, l := range page {
			if seen[l.Token] {
				t.Errorf("token %q returned on more than one page", l.Token)
			}
			seen[l.Token] = true
		}
	}
	if len(seen) != total {
		t.Errorf("paginated over %d distinct tokens, want %d", len(seen), total)
	}

	// Offset past the end yields an empty page.
	if page, _ := store.ListUserListings(ctx, 100, 2, total); len(page) != 0 {
		t.Errorf("offset past end = %d rows, want 0", len(page))
	}
}

func TestPostgres_DropListingByToken(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)
	seedPgUser(t, store, 200)

	save := func(token string, chatID int64) {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: token, ChatID: chatID, SearchName: "s",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
			FirstSeenAt: time.Now(),
		}); err != nil {
			t.Fatalf("save %s/%d: %v", token, chatID, err)
		}
	}

	// "gone-1" is held by two different chats, none bookmarked.
	save("gone-1", 100)
	save("gone-1", 200)
	// "gone-2" is bookmarked by chat 100.
	save("gone-2", 100)
	if err := store.SaveBookmark(ctx, 100, "gone-2"); err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}

	// Non-bookmarked copies are hard-deleted across every chat that held them.
	deleted, err := store.DropListingByToken(ctx, "gone-1")
	if err != nil {
		t.Fatalf("DropListingByToken gone-1: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2 (both chats)", deleted)
	}
	if l, _ := store.GetListing(ctx, 100, "gone-1"); l != nil {
		t.Error("gone-1 should be deleted for chat 100")
	}
	if l, _ := store.GetListing(ctx, 200, "gone-1"); l != nil {
		t.Error("gone-1 should be deleted for chat 200")
	}

	// Bookmarked copies are preserved but flagged removed_at ("likely sold").
	deleted, err = store.DropListingByToken(ctx, "gone-2")
	if err != nil {
		t.Fatalf("DropListingByToken gone-2: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0 (bookmarked is preserved)", deleted)
	}
	l, _ := store.GetListing(ctx, 100, "gone-2")
	if l == nil {
		t.Fatal("bookmarked gone-2 should be preserved, not deleted")
	}
	if l.RemovedAt == nil {
		t.Error("bookmarked gone-2 should be flagged removed_at")
	}

	// Unknown token is a no-op.
	if deleted, err := store.DropListingByToken(ctx, "never-existed"); err != nil || deleted != 0 {
		t.Errorf("drop unknown token = %d, %v; want 0, nil", deleted, err)
	}
}

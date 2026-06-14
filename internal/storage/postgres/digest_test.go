package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestPostgres_DailyStats(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "daily", Manufacturer: 1, Model: 1,
	})
	if err != nil {
		t.Fatalf("CreateSearch: %v", err)
	}

	// Fewer than the HAVING COUNT(*) >= 5 threshold yields no rows.
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "ds-x", ChatID: 100, SearchID: searchID, SearchName: "daily",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
		FirstSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	stats, err := store.DailyStats(ctx, 100)
	if err != nil {
		t.Fatalf("DailyStats below threshold: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("below threshold = %d rows, want 0", len(stats))
	}

	// Cross the threshold: 6 priced listings on the same search.
	now := time.Now()
	for i := 0; i < 6; i++ {
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token:        "ds-" + string(rune('a'+i)),
			ChatID:       100,
			SearchID:     searchID,
			SearchName:   "daily",
			Manufacturer: "Toyota",
			Model:        "Corolla",
			Year:         2020,
			Price:        70000 + i*1000,
			PageLink:     "https://example.com/" + string(rune('a'+i)),
			FirstSeenAt:  now,
		}); err != nil {
			t.Fatalf("seed listing %d: %v", i, err)
		}
	}

	stats, err = store.DailyStats(ctx, 100)
	if err != nil {
		t.Fatalf("DailyStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 search row, got %d", len(stats))
	}
	if stats[0].SearchName != "daily" {
		t.Errorf("search name = %q, want daily", stats[0].SearchName)
	}
	if stats[0].BestPrice != 70000 {
		t.Errorf("best price = %d, want 70000", stats[0].BestPrice)
	}
	if stats[0].NewCount < 6 {
		t.Errorf("new count = %d, want >= 6", stats[0].NewCount)
	}
}

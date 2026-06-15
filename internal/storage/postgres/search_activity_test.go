package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestPostgres_SearchDailyCounts(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	seedPgUser(t, store, 100)

	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "trend", Manufacturer: 1, Model: 1,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	now := time.Now()
	save := func(token string, daysAgo int) {
		t.Helper()
		if err := store.SaveListing(ctx, storage.ListingRecord{
			Token: token, ChatID: 100, SearchID: searchID, SearchName: "trend",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 80000,
			FirstSeenAt: now.AddDate(0, 0, -daysAgo),
		}); err != nil {
			t.Fatalf("save %s: %v", token, err)
		}
	}
	save("d-a", 0)
	save("d-b", 0)
	save("d-c", 1)
	save("d-old", 40) // outside the 14-day window

	got, err := store.SearchDailyCounts(ctx, 100, searchID, 14)
	if err != nil {
		t.Fatalf("SearchDailyCounts: %v", err)
	}
	// Dense series: exactly 14 day buckets, all labelled.
	if len(got) != 14 {
		t.Fatalf("len(got) = %d, want 14 dense buckets", len(got))
	}
	total := 0
	for _, d := range got {
		total += d.Count
		if d.Day == "" {
			t.Errorf("empty day bucket in %+v", got)
		}
	}
	if total != 3 {
		t.Fatalf("total in 14d window = %d, want 3 (rows: %+v)", total, got)
	}

	// A different search has data on no day (dense zero series).
	other, err := store.SearchDailyCounts(ctx, 100, searchID+9999, 14)
	if err != nil {
		t.Fatalf("other: %v", err)
	}
	otherTotal := 0
	for _, d := range other {
		otherTotal += d.Count
	}
	if otherTotal != 0 {
		t.Fatalf("other search total = %d, want 0", otherTotal)
	}

	// days is clamped; an absurd value falls back to the default window.
	if _, err := store.SearchDailyCounts(ctx, 100, searchID, 9999); err != nil {
		t.Fatalf("clamped days: %v", err)
	}
}

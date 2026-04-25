package bot

import (
	"context"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestPauseSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 200, "bob")

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 200, Name: "test-pause", Manufacturer: 1, Model: 1,
		YearMin: 2020, YearMax: 2024, PriceMax: 100000,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	// Search starts active
	s, err := store.GetSearch(ctx, id)
	if err != nil {
		t.Fatalf("get search: %v", err)
	}
	if !s.Active {
		t.Error("new search should be active")
	}

	// Pause it
	if err := store.SetSearchActive(ctx, id, 200, false); err != nil {
		t.Fatalf("pause search: %v", err)
	}

	s, err = store.GetSearch(ctx, id)
	if err != nil {
		t.Fatalf("get search after pause: %v", err)
	}
	if s.Active {
		t.Error("search should be paused after SetSearchActive(false)")
	}

	// Paused searches should not appear in active count
	count, _ := store.CountSearches(ctx, 200)
	if count != 0 {
		t.Errorf("active count = %d, want 0 (search is paused)", count)
	}

	// Paused searches should not appear in ListAllActiveSearches
	active, _ := store.ListAllActiveSearches(ctx)
	for _, a := range active {
		if a.ID == id {
			t.Error("paused search should not appear in ListAllActiveSearches")
		}
	}
}

func TestResumeSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 300, "carol")

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 300, Name: "test-resume", Manufacturer: 2, Model: 1,
		YearMin: 2019, YearMax: 2023, PriceMax: 200000,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	// Pause then resume
	if err := store.SetSearchActive(ctx, id, 300, false); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := store.SetSearchActive(ctx, id, 300, true); err != nil {
		t.Fatalf("resume: %v", err)
	}

	s, err := store.GetSearch(ctx, id)
	if err != nil {
		t.Fatalf("get search after resume: %v", err)
	}
	if !s.Active {
		t.Error("search should be active after resume")
	}

	// Resumed search should appear in active count
	count, _ := store.CountSearches(ctx, 300)
	if count != 1 {
		t.Errorf("active count = %d, want 1", count)
	}
}

func TestPauseSearchOwnership(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 400, "dave")
	_ = store.UpsertUser(ctx, 401, "eve")

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 400, Name: "dave-search", Manufacturer: 3, Model: 1,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	// Verify ownership check: GetSearch returns the search but chatID won't match
	s, err := store.GetSearch(ctx, id)
	if err != nil {
		t.Fatalf("get search: %v", err)
	}
	if s.ChatID != 400 {
		t.Errorf("search owner = %d, want 400", s.ChatID)
	}

	// Eve's chatID (401) doesn't match - the handler would reject this
	if s.ChatID == 401 {
		t.Error("search should not belong to eve")
	}
}

func TestPauseAlreadyPausedSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 500, "frank")

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 500, Name: "test-double-pause", Manufacturer: 4, Model: 1,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	// Pause it
	if err := store.SetSearchActive(ctx, id, 500, false); err != nil {
		t.Fatalf("first pause: %v", err)
	}

	// Pause again - should succeed (idempotent)
	if err := store.SetSearchActive(ctx, id, 500, false); err != nil {
		t.Fatalf("second pause: %v", err)
	}

	s, _ := store.GetSearch(ctx, id)
	if s.Active {
		t.Error("search should still be paused")
	}
}

func TestResumeAlreadyActiveSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 600, "grace")

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 600, Name: "test-double-resume", Manufacturer: 5, Model: 1,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	// Resume an already active search - should succeed (idempotent)
	if err := store.SetSearchActive(ctx, id, 600, true); err != nil {
		t.Fatalf("resume active search: %v", err)
	}

	s, _ := store.GetSearch(ctx, id)
	if !s.Active {
		t.Error("search should still be active")
	}
}

func TestListSearchesShowsPausedStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 700, "heidi")

	id1, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 700, Name: "active-search", Manufacturer: 1, Model: 1,
	})
	id2, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 700, Name: "paused-search", Manufacturer: 2, Model: 1,
	})

	// Pause the second search
	_ = store.SetSearchActive(ctx, id2, 700, false)

	searches, err := store.ListSearches(ctx, 700)
	if err != nil {
		t.Fatalf("list searches: %v", err)
	}
	if len(searches) != 2 {
		t.Fatalf("expected 2 searches, got %d", len(searches))
	}

	// Verify both searches are returned with correct active status
	statusByID := make(map[int64]bool)
	for _, s := range searches {
		statusByID[s.ID] = s.Active
	}

	if !statusByID[id1] {
		t.Error("search 1 should be active")
	}
	if statusByID[id2] {
		t.Error("search 2 should be paused")
	}
}

func TestGetSearchNonExistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s, err := store.GetSearch(ctx, 99999)
	if err != nil {
		t.Fatalf("get non-existent search: %v", err)
	}
	if s != nil {
		t.Error("expected nil for non-existent search")
	}
}

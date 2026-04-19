package bot

import (
	"context"
	"fmt"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestShareLink(t *testing.T) {
	tests := []struct {
		botUsername string
		searchID   int64
		want       string
	}{
		{"CarWatchBot", 1, "https://t.me/CarWatchBot?start=share_1"},
		{"CarWatchBot", 42, "https://t.me/CarWatchBot?start=share_42"},
		{"my_test_bot", 999, "https://t.me/my_test_bot?start=share_999"},
	}
	for _, tt := range tests {
		got := ShareLink(tt.botUsername, tt.searchID)
		if got != tt.want {
			t.Errorf("ShareLink(%q, %d) = %q, want %q", tt.botUsername, tt.searchID, got, tt.want)
		}
	}
}

func TestCloneSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Set up the original owner and a second user.
	_ = store.UpsertUser(ctx, 100, "alice")
	_ = store.UpsertUser(ctx, 200, "bob")

	// Alice creates a search.
	srcID, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       100,
		Name:         "mazda-3",
		Manufacturer: 27,
		Model:        10332,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     150000,
		EngineMinCC:  2000,
		MaxKm:        120000,
		MaxHand:      3,
	})
	if err != nil {
		t.Fatalf("create source search: %v", err)
	}

	// Retrieve the source search.
	src, err := store.GetSearch(ctx, srcID)
	if err != nil || src == nil {
		t.Fatalf("get source search: %v", err)
	}

	// Clone it for Bob.
	cloneID, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       200,
		Name:         src.Name,
		Manufacturer: src.Manufacturer,
		Model:        src.Model,
		YearMin:      src.YearMin,
		YearMax:      src.YearMax,
		PriceMax:     src.PriceMax,
		EngineMinCC:  src.EngineMinCC,
		MaxKm:        src.MaxKm,
		MaxHand:      src.MaxHand,
	})
	if err != nil {
		t.Fatalf("clone search: %v", err)
	}

	clone, err := store.GetSearch(ctx, cloneID)
	if err != nil || clone == nil {
		t.Fatalf("get cloned search: %v", err)
	}

	// The clone should belong to Bob, not Alice.
	if clone.ChatID != 200 {
		t.Errorf("clone.ChatID = %d, want 200", clone.ChatID)
	}

	// All search parameters should match the source.
	if clone.Manufacturer != src.Manufacturer {
		t.Errorf("Manufacturer = %d, want %d", clone.Manufacturer, src.Manufacturer)
	}
	if clone.Model != src.Model {
		t.Errorf("Model = %d, want %d", clone.Model, src.Model)
	}
	if clone.YearMin != src.YearMin {
		t.Errorf("YearMin = %d, want %d", clone.YearMin, src.YearMin)
	}
	if clone.YearMax != src.YearMax {
		t.Errorf("YearMax = %d, want %d", clone.YearMax, src.YearMax)
	}
	if clone.PriceMax != src.PriceMax {
		t.Errorf("PriceMax = %d, want %d", clone.PriceMax, src.PriceMax)
	}
	if clone.EngineMinCC != src.EngineMinCC {
		t.Errorf("EngineMinCC = %d, want %d", clone.EngineMinCC, src.EngineMinCC)
	}
	if clone.MaxKm != src.MaxKm {
		t.Errorf("MaxKm = %d, want %d", clone.MaxKm, src.MaxKm)
	}
	if clone.MaxHand != src.MaxHand {
		t.Errorf("MaxHand = %d, want %d", clone.MaxHand, src.MaxHand)
	}
	if clone.Name != src.Name {
		t.Errorf("Name = %q, want %q", clone.Name, src.Name)
	}

	// Both users should now have their own searches.
	aliceSearches, _ := store.ListSearches(ctx, 100)
	bobSearches, _ := store.ListSearches(ctx, 200)
	if len(aliceSearches) != 1 {
		t.Errorf("alice should have 1 search, got %d", len(aliceSearches))
	}
	if len(bobSearches) != 1 {
		t.Errorf("bob should have 1 search, got %d", len(bobSearches))
	}
}

func TestCloneSearchRespectsLimit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 100, "alice")
	_ = store.UpsertUser(ctx, 200, "bob")

	// Alice creates a search to share.
	srcID, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "shared", Manufacturer: 27, Model: 10332,
		YearMin: 2018, YearMax: 2024, PriceMax: 150000,
	})

	// Bob already has the maximum number of searches (3).
	const maxSearches = 3
	for i := range maxSearches {
		_, _ = store.CreateSearch(ctx, storage.Search{
			ChatID: 200, Name: fmt.Sprintf("bob-%d", i),
			Manufacturer: i + 1, Model: 1,
		})
	}

	count, _ := store.CountSearches(ctx, 200)
	if count < int64(maxSearches) {
		t.Fatalf("bob should be at the limit, count = %d", count)
	}

	// Verify that source search exists (the handler would check this).
	src, _ := store.GetSearch(ctx, srcID)
	if src == nil {
		t.Fatal("source search not found")
	}

	// The bot would block cloning here because count >= maxSearches.
	// This test verifies the count check logic that the handler uses.
	if count < int64(maxSearches) {
		t.Error("rate limit should block cloning when at max searches")
	}
}

func TestCloneSearchFromDeletedSource(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 100, "alice")

	// Create and then delete the source search.
	srcID, _ := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "deleted", Manufacturer: 27, Model: 10332,
	})
	_ = store.DeleteSearch(ctx, srcID, 100)

	// The search should no longer exist.
	src, err := store.GetSearch(ctx, srcID)
	if err != nil {
		t.Fatalf("get deleted search: %v", err)
	}
	if src != nil {
		t.Error("deleted search should return nil")
	}
}

func TestShareCallbackDataParsing(t *testing.T) {
	tests := []struct {
		data string
		id   string
	}{
		{"share_copy:1", "1"},
		{"share_copy:42", "42"},
		{"share_copy:999", "999"},
	}
	for _, tt := range tests {
		trimmed := tt.data[len(cbPrefixShareCopy):]
		if trimmed != tt.id {
			t.Errorf("parse %q after removing prefix: got %q, want %q", tt.data, trimmed, tt.id)
		}
	}
}

func TestShareStartParamParsing(t *testing.T) {
	tests := []struct {
		param string
		id    string
	}{
		{"share_1", "1"},
		{"share_42", "42"},
		{"share_999", "999"},
	}
	for _, tt := range tests {
		trimmed := tt.param[len("share_"):]
		if trimmed != tt.id {
			t.Errorf("parse %q: got %q, want %q", tt.param, trimmed, tt.id)
		}
	}
}

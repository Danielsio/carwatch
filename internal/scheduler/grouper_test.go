package scheduler

import (
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestGroupSearches_SingleGroup(t *testing.T) {
	searches := []storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024, PriceMax: 150000},
		{ID: 2, ChatID: 200, Manufacturer: 27, Model: 10332, YearMin: 2020, YearMax: 2026, PriceMax: 200000},
	}

	groups := GroupSearches(searches)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if g.Params.YearMin != 2018 {
		t.Errorf("YearMin = %d, want 2018 (min of all users)", g.Params.YearMin)
	}
	if g.Params.YearMax != 2026 {
		t.Errorf("YearMax = %d, want 2026 (max of all users)", g.Params.YearMax)
	}
	if g.Params.PriceMax != 200000 {
		t.Errorf("PriceMax = %d, want 200000 (max of all users)", g.Params.PriceMax)
	}
	if len(g.Searches) != 2 {
		t.Errorf("expected 2 searches in group, got %d", len(g.Searches))
	}
}

func TestGroupSearches_MultipleGroups(t *testing.T) {
	searches := []storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332},
		{ID: 2, ChatID: 200, Manufacturer: 35, Model: 10471},
		{ID: 3, ChatID: 300, Manufacturer: 27, Model: 10332},
	}

	groups := GroupSearches(searches)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (Mazda 3 + Toyota Corolla), got %d", len(groups))
	}
}

func TestGroupSearches_Empty(t *testing.T) {
	groups := GroupSearches(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

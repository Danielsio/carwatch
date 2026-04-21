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

func TestGroupSearches_GroupBySource(t *testing.T) {
	searches := []storage.Search{
		{ID: 1, ChatID: 100, Source: "yad2", Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024, PriceMax: 150000},
		{ID: 2, ChatID: 200, Source: "winwin", Manufacturer: 27, Model: 10332, YearMin: 2020, YearMax: 2026, PriceMax: 200000},
		{ID: 3, ChatID: 300, Source: "yad2", Manufacturer: 27, Model: 10332, YearMin: 2019, YearMax: 2025, PriceMax: 180000},
	}

	groups := GroupSearches(searches)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (same car, different sources), got %d", len(groups))
	}

	// Find each group by source.
	var yad2Group, winwinGroup *CanonicalGroup
	for i := range groups {
		switch groups[i].Source {
		case "yad2":
			yad2Group = &groups[i]
		case "winwin":
			winwinGroup = &groups[i]
		}
	}

	if yad2Group == nil || winwinGroup == nil {
		t.Fatal("expected one yad2 group and one winwin group")
	}

	if len(yad2Group.Searches) != 2 {
		t.Errorf("yad2 group has %d searches, want 2", len(yad2Group.Searches))
	}
	if len(winwinGroup.Searches) != 1 {
		t.Errorf("winwin group has %d searches, want 1", len(winwinGroup.Searches))
	}

	// Verify yad2 group merged params correctly.
	if yad2Group.Params.YearMin != 2018 {
		t.Errorf("yad2 YearMin = %d, want 2018", yad2Group.Params.YearMin)
	}
	if yad2Group.Params.YearMax != 2025 {
		t.Errorf("yad2 YearMax = %d, want 2025", yad2Group.Params.YearMax)
	}
	if yad2Group.Params.PriceMax != 180000 {
		t.Errorf("yad2 PriceMax = %d, want 180000", yad2Group.Params.PriceMax)
	}
}

func TestGroupSearches_MultiSource(t *testing.T) {
	searches := []storage.Search{
		{ID: 1, ChatID: 100, Source: "yad2,winwin", Manufacturer: 27, Model: 10332, YearMin: 2018, YearMax: 2024, PriceMax: 150000},
	}

	groups := GroupSearches(searches)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (one per source), got %d", len(groups))
	}

	var yad2, winwin *CanonicalGroup
	for i := range groups {
		switch groups[i].Source {
		case "yad2":
			yad2 = &groups[i]
		case "winwin":
			winwin = &groups[i]
		}
	}

	if yad2 == nil || winwin == nil {
		t.Fatal("expected one yad2 and one winwin group")
	}
	if len(yad2.Searches) != 1 || len(winwin.Searches) != 1 {
		t.Error("each group should have 1 search")
	}
}

func TestGroupSearches_EmptySourceDefaultsToYad2(t *testing.T) {
	searches := []storage.Search{
		{ID: 1, ChatID: 100, Source: "", Manufacturer: 27, Model: 10332},
		{ID: 2, ChatID: 200, Source: "yad2", Manufacturer: 27, Model: 10332},
	}

	groups := GroupSearches(searches)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (empty source defaults to yad2), got %d", len(groups))
	}
	if groups[0].Source != "yad2" {
		t.Errorf("Source = %q, want %q", groups[0].Source, "yad2")
	}
	if len(groups[0].Searches) != 2 {
		t.Errorf("expected 2 searches in group, got %d", len(groups[0].Searches))
	}
}

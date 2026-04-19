package bot

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestWizardStateFlow(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 100, "alice")

	_ = store.UpdateUserState(ctx, 100, StateAskManufacturer, "{}")
	u, _ := store.GetUser(ctx, 100)
	if u.State != StateAskManufacturer {
		t.Errorf("state = %q, want %q", u.State, StateAskManufacturer)
	}

	wd := WizardData{Manufacturer: 27, ManufacturerName: "Mazda"}
	data, _ := json.Marshal(wd)
	_ = store.UpdateUserState(ctx, 100, StateAskModel, string(data))

	u, _ = store.GetUser(ctx, 100)
	if u.State != StateAskModel {
		t.Errorf("state = %q, want %q", u.State, StateAskModel)
	}

	var loaded WizardData
	_ = json.Unmarshal([]byte(u.StateData), &loaded)
	if loaded.Manufacturer != 27 || loaded.ManufacturerName != "Mazda" {
		t.Errorf("wizard data = %+v", loaded)
	}
}

func TestWizardCompleteFlow(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 100, "alice")

	wd := WizardData{
		Manufacturer:     27,
		ManufacturerName: "Mazda",
		Model:            10332,
		ModelName:        "3",
		YearMin:          2018,
		YearMax:          2024,
		PriceMax:         150000,
		EngineMinCC:      2000,
	}

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       100,
		Name:         "mazda-3",
		Manufacturer: wd.Manufacturer,
		Model:        wd.Model,
		YearMin:      wd.YearMin,
		YearMax:      wd.YearMax,
		PriceMax:     wd.PriceMax,
		EngineMinCC:  wd.EngineMinCC,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero search ID")
	}

	_ = store.UpdateUserState(ctx, 100, StateIdle, "{}")
	u, _ := store.GetUser(ctx, 100)
	if u.State != StateIdle {
		t.Errorf("state = %q after confirm, want idle", u.State)
	}

	searches, _ := store.ListSearches(ctx, 100)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Name != "mazda-3" || searches[0].PriceMax != 150000 {
		t.Errorf("search = %+v", searches[0])
	}
}

func TestLoadWizardData_Empty(t *testing.T) {
	var wd WizardData
	_ = json.Unmarshal([]byte("{}"), &wd)
	if wd.Manufacturer != 0 || wd.YearMin != 0 {
		t.Errorf("empty wizard data should have zero values: %+v", wd)
	}
}

func TestLoadWizardData_Populated(t *testing.T) {
	data := `{"manufacturer":27,"manufacturer_name":"Mazda","model":10332,"model_name":"3","year_min":2018}`
	var wd WizardData
	if err := json.Unmarshal([]byte(data), &wd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wd.Manufacturer != 27 || wd.YearMin != 2018 {
		t.Errorf("wizard data = %+v", wd)
	}
}

func TestWizardData_Source(t *testing.T) {
	data := `{"source":"winwin","manufacturer":27,"manufacturer_name":"Mazda"}`
	var wd WizardData
	if err := json.Unmarshal([]byte(data), &wd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wd.Source != "winwin" {
		t.Errorf("Source = %q, want %q", wd.Source, "winwin")
	}
	if wd.Manufacturer != 27 {
		t.Errorf("Manufacturer = %d, want 27", wd.Manufacturer)
	}
}

func TestWizardCompleteFlow_WithSource(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 100, "alice")

	wd := WizardData{
		Source:           "winwin",
		Manufacturer:     27,
		ManufacturerName: "Mazda",
		Model:            10332,
		ModelName:        "3",
		YearMin:          2018,
		YearMax:          2024,
		PriceMax:         150000,
		EngineMinCC:      2000,
	}

	id, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       100,
		Name:         "mazda-3",
		Source:       wd.Source,
		Manufacturer: wd.Manufacturer,
		Model:        wd.Model,
		YearMin:      wd.YearMin,
		YearMax:      wd.YearMax,
		PriceMax:     wd.PriceMax,
		EngineMinCC:  wd.EngineMinCC,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero search ID")
	}

	searches, _ := store.ListSearches(ctx, 100)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Source != "winwin" {
		t.Errorf("Source = %q, want %q", searches[0].Source, "winwin")
	}
}

func TestSourceDefaultsToYad2(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 100, "alice")

	// Create search with empty source - should default to yad2.
	_, err := store.CreateSearch(ctx, storage.Search{
		ChatID:       100,
		Name:         "mazda-3",
		Manufacturer: 27,
		Model:        10332,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	searches, _ := store.ListSearches(ctx, 100)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Source != "yad2" {
		t.Errorf("Source = %q, want %q (default)", searches[0].Source, "yad2")
	}
}

func TestSearchRateLimit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.UpsertUser(ctx, 100, "alice")

	for i := range 3 {
		_, err := store.CreateSearch(ctx, storage.Search{
			ChatID: 100, Name: "test", Manufacturer: i + 1, Model: 1,
		})
		if err != nil {
			t.Fatalf("create search %d: %v", i, err)
		}
	}

	count, _ := store.CountSearches(ctx, 100)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

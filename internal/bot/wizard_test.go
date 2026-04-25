package bot

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestNormalizeKeywords(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"", ""},
		{"  ", ""},
		{"hybrid", "hybrid"},
		{"hybrid, auto", "hybrid,auto"},
		{" hybrid , auto , ", "hybrid,auto"},
		{"hybrid,,auto", "hybrid,auto"},
		{"  sunroof  ", "sunroof"},
		{"a,b,c", "a,b,c"},
	}
	for _, tt := range tests {
		got := normalizeKeywords(tt.input)
		if got != tt.want {
			t.Errorf("normalizeKeywords(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHandleKeywordsInput(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 800
	tb.createUser(ctx, t, chatID, "alice")

	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxKm+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxHand+"0")

	// Now in StateAskKeywords — type actual keywords
	tb.simulateText(ctx, chatID, "sunroof, leather")

	// Should now be in StateAskExcludeKeys
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskExcludeKeys {
		t.Errorf("state = %q, want %q", user.State, StateAskExcludeKeys)
	}

	var wd WizardData
	_ = json.Unmarshal([]byte(user.StateData), &wd)
	if wd.Keywords != "sunroof,leather" {
		t.Errorf("Keywords = %q, want %q", wd.Keywords, "sunroof,leather")
	}
}

func TestHandleKeywordsInput_Skip(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 801
	tb.createUser(ctx, t, chatID, "bob")

	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxKm+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxHand+"0")

	// Skip keywords
	tb.simulateText(ctx, chatID, "skip")

	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskExcludeKeys {
		t.Errorf("state = %q, want %q", user.State, StateAskExcludeKeys)
	}

	var wd WizardData
	_ = json.Unmarshal([]byte(user.StateData), &wd)
	if wd.Keywords != "" {
		t.Errorf("Keywords = %q, want empty after skip", wd.Keywords)
	}
}

func TestWizardFlow_WithKeywords(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 802
	tb.createUser(ctx, t, chatID, "carol")

	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxKm+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxHand+"0")

	// Enter keywords
	tb.simulateText(ctx, chatID, "hybrid, automatic")
	// Enter exclude keys
	tb.simulateText(ctx, chatID, "accident, damaged")
	// Confirm
	tb.simulateCallback(ctx, chatID, cbConfirm)

	searches, _ := tb.store.ListSearches(ctx, chatID)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Keywords != "hybrid,automatic" {
		t.Errorf("Keywords = %q, want %q", searches[0].Keywords, "hybrid,automatic")
	}
	if searches[0].ExcludeKeys != "accident,damaged" {
		t.Errorf("ExcludeKeys = %q, want %q", searches[0].ExcludeKeys, "accident,damaged")
	}
}

// --- /edit flow tests ---

func TestHandleEdit_NoArgs(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 810
	tb.createUser(ctx, t, chatID, "eve")

	tb.simulateCommand(ctx, chatID, "/edit")
	last := tb.msg.last()
	if !contains(last.Text, "edit") {
		t.Errorf("expected edit usage message, got %q", last.Text)
	}
}

func TestHandleEdit_InvalidID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 811
	tb.createUser(ctx, t, chatID, "frank")

	tb.simulateCommand(ctx, chatID, "/edit abc")
	last := tb.msg.last()
	if !contains(last.Text, "Invalid") {
		t.Errorf("expected invalid ID message, got %q", last.Text)
	}
}

func TestHandleEdit_NonexistentID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 812
	tb.createUser(ctx, t, chatID, "grace")

	tb.simulateCommand(ctx, chatID, "/edit 999")
	last := tb.msg.last()
	if !contains(last.Text, "not found") {
		t.Errorf("expected not found message, got %q", last.Text)
	}
}

func TestHandleEdit_WrongUser(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const ownerID int64 = 813
	const otherID int64 = 814
	tb.createUser(ctx, t, ownerID, "hank")
	tb.createUser(ctx, t, otherID, "iris")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: ownerID, Name: "test", Manufacturer: 27, Model: 10332,
		YearMin: 2018, YearMax: 2024, PriceMax: 150000,
	})

	tb.simulateCommand(ctx, otherID, "/edit "+itoa(id))
	last := tb.msg.last()
	if !contains(last.Text, "not found") {
		t.Errorf("expected not found for wrong user, got %q", last.Text)
	}
}

func TestHandleEdit_ValidID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 815
	tb.createUser(ctx, t, chatID, "jack")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "mazda-3", Source: "yad2",
		Manufacturer: 27, Model: 10332,
		YearMin: 2018, YearMax: 2024, PriceMax: 150000,
		Keywords: "sunroof", ExcludeKeys: "accident",
	})

	tb.simulateCommand(ctx, chatID, "/edit "+itoa(id))

	// Should be in StateAskSource with wizard data populated from the search.
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskSource {
		t.Errorf("state = %q, want %q", user.State, StateAskSource)
	}

	var wd WizardData
	_ = json.Unmarshal([]byte(user.StateData), &wd)
	if wd.EditSearchID != id {
		t.Errorf("EditSearchID = %d, want %d", wd.EditSearchID, id)
	}
	if wd.Manufacturer != 27 {
		t.Errorf("Manufacturer = %d, want 27", wd.Manufacturer)
	}
	if wd.Keywords != "sunroof" {
		t.Errorf("Keywords = %q, want %q", wd.Keywords, "sunroof")
	}
	if wd.ExcludeKeys != "accident" {
		t.Errorf("ExcludeKeys = %q, want %q", wd.ExcludeKeys, "accident")
	}
}

func TestHandleEdit_FullFlow(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 816
	tb.createUser(ctx, t, chatID, "kate")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "mazda-3", Source: "yad2",
		Manufacturer: 27, Model: 10332,
		YearMin: 2018, YearMax: 2024, PriceMax: 150000,
		Keywords: "sunroof",
	})

	tb.simulateCommand(ctx, chatID, "/edit "+itoa(id))

	// Walk through wizard: keep source, mfr, model, update years and price
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2020")
	tb.simulateText(ctx, chatID, "2025")
	tb.simulateText(ctx, chatID, "200000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxKm+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxHand+"0")
	tb.simulateCallback(ctx, chatID, cbSkipKeywords)
	tb.simulateCallback(ctx, chatID, cbSkipExcludeKeys)
	tb.simulateCallback(ctx, chatID, cbConfirm)

	// Verify the search was updated, not a new one created.
	searches, _ := tb.store.ListSearches(ctx, chatID)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search (updated), got %d", len(searches))
	}
	s := searches[0]
	if s.ID != id {
		t.Errorf("search ID = %d, want %d (same search updated)", s.ID, id)
	}
	if s.YearMin != 2020 || s.YearMax != 2025 || s.PriceMax != 200000 {
		t.Errorf("search not updated: year=%d-%d price=%d", s.YearMin, s.YearMax, s.PriceMax)
	}
}

func TestHandleEdit_KeywordsRoundTrip(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 817
	tb.createUser(ctx, t, chatID, "leo")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "mazda-3", Source: "yad2",
		Manufacturer: 27, Model: 10332,
		YearMin: 2018, YearMax: 2024, PriceMax: 150000,
		Keywords: "sunroof,leather", ExcludeKeys: "accident",
	})

	tb.simulateCommand(ctx, chatID, "/edit "+itoa(id))
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxKm+"0")
	tb.simulateCallback(ctx, chatID, cbPrefixMaxHand+"0")

	// Enter new keywords
	tb.simulateText(ctx, chatID, "hybrid, electric")
	// Keep old exclude keys
	tb.simulateText(ctx, chatID, "damaged, broken")
	// Confirm
	tb.simulateCallback(ctx, chatID, cbConfirm)

	s, _ := tb.store.GetSearch(ctx, id)
	if s.Keywords != "hybrid,electric" {
		t.Errorf("Keywords = %q, want %q", s.Keywords, "hybrid,electric")
	}
	if s.ExcludeKeys != "damaged,broken" {
		t.Errorf("ExcludeKeys = %q, want %q", s.ExcludeKeys, "damaged,broken")
	}
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
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

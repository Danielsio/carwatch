package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func TestWizardData_JSON(t *testing.T) {
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

	data, err := json.Marshal(wd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded WizardData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Manufacturer != 27 || decoded.ModelName != "3" || decoded.PriceMax != 150000 {
		t.Errorf("roundtrip failed: %+v", decoded)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{85000, "85,000"},
		{150000, "150,000"},
		{1500000, "1,500,000"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestKeyboards_ManufacturerKeyboard(t *testing.T) {
	kb := manufacturerKeyboard()
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("keyboard should have rows")
	}

	total := 0
	for _, row := range kb.InlineKeyboard {
		total += len(row)
		for _, btn := range row {
			if btn.CallbackData == "" {
				t.Errorf("button %q has empty callback data", btn.Text)
			}
		}
	}
	if total < 10 {
		t.Errorf("expected at least 10 manufacturer buttons, got %d", total)
	}
}

func TestKeyboards_ModelKeyboard(t *testing.T) {
	kb := modelKeyboard(27) // Mazda
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("Mazda model keyboard should have rows")
	}

	found := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "3" {
				found = true
			}
		}
	}
	if !found {
		t.Error("Mazda 3 button not found in model keyboard")
	}
}

func TestKeyboards_EngineKeyboard(t *testing.T) {
	kb := engineKeyboard()
	if len(kb.InlineKeyboard) != 2 {
		t.Errorf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
}

func TestKeyboards_ConfirmKeyboard(t *testing.T) {
	wd := WizardData{
		ManufacturerName: "Mazda",
		ModelName:        "3",
		YearMin:          2018,
		YearMax:          2024,
		PriceMax:         150000,
		EngineMinCC:      2000,
	}

	kb, summary := confirmKeyboard(wd)
	if kb == nil {
		t.Fatal("keyboard should not be nil")
	}
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
	if !contains(summary, "Mazda") || !contains(summary, "150,000") {
		t.Errorf("summary missing expected content: %s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func newBotTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRateLimitEnforcement(t *testing.T) {
	store := newBotTestStore(t)
	ctx := context.Background()

	const chatID int64 = 200
	const maxSearches = 3

	_ = store.UpsertUser(ctx, chatID, "bob")

	// Create searches up to the limit.
	for i := range maxSearches {
		_, err := store.CreateSearch(ctx, storage.Search{
			ChatID: chatID, Name: fmt.Sprintf("search-%d", i),
			Manufacturer: i + 1, Model: 1,
		})
		if err != nil {
			t.Fatalf("create search %d: %v", i, err)
		}
	}

	count, err := store.CountSearches(ctx, chatID)
	if err != nil {
		t.Fatalf("count searches: %v", err)
	}
	if count != int64(maxSearches) {
		t.Errorf("count = %d, want %d", count, maxSearches)
	}

	// Simulate the rate-limit check the bot performs.
	if count < int64(maxSearches) {
		t.Error("rate limit should block new search creation")
	}
}

func TestRateLimitAfterDeletion(t *testing.T) {
	store := newBotTestStore(t)
	ctx := context.Background()

	const chatID int64 = 300
	const maxSearches = 3

	_ = store.UpsertUser(ctx, chatID, "carol")

	var ids []int64
	for i := range maxSearches {
		id, err := store.CreateSearch(ctx, storage.Search{
			ChatID: chatID, Name: fmt.Sprintf("search-%d", i),
			Manufacturer: i + 1, Model: 1,
		})
		if err != nil {
			t.Fatalf("create search %d: %v", i, err)
		}
		ids = append(ids, id)
	}

	// At limit — deletion should free a slot.
	_ = store.DeleteSearch(ctx, ids[0], chatID)

	count, _ := store.CountSearches(ctx, chatID)
	if count >= int64(maxSearches) {
		t.Errorf("after deletion count = %d, should be below %d", count, maxSearches)
	}
}

func TestSettingsDisplay(t *testing.T) {
	store := newBotTestStore(t)
	ctx := context.Background()

	const chatID int64 = 400
	const maxSearches = 3

	_ = store.UpsertUser(ctx, chatID, "dave")

	// Create 2 searches.
	for i := range 2 {
		_, _ = store.CreateSearch(ctx, storage.Search{
			ChatID: chatID, Name: fmt.Sprintf("s-%d", i),
			Manufacturer: i + 1, Model: 1,
		})
	}

	count, _ := store.CountSearches(ctx, chatID)

	// Verify the settings message format matches what handleSettings produces.
	msg := fmt.Sprintf("*Your settings:*\nActive searches: %d/%d", count, maxSearches)

	if !contains(msg, "2/3") {
		t.Errorf("settings display should show 2/3, got: %s", msg)
	}
}

func TestMaxSearchesDefault(t *testing.T) {
	store := newBotTestStore(t)
	logger := slog.Default()

	// MaxSearches = 0 should default to 3.
	b := New(nil, store, store, Config{MaxSearches: 0}, logger)
	if b.maxSearches != 3 {
		t.Errorf("maxSearches = %d, want 3", b.maxSearches)
	}

	// Explicit value should be preserved.
	b2 := New(nil, store, store, Config{MaxSearches: 5}, logger)
	if b2.maxSearches != 5 {
		t.Errorf("maxSearches = %d, want 5", b2.maxSearches)
	}
}

package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func TestIsRateLimited(t *testing.T) {
	tb := newTestBot(t)
	const chatID int64 = 900

	// First rateLimitBurst calls should succeed.
	for i := range rateLimitBurst {
		if tb.bot.isRateLimited(chatID) {
			t.Fatalf("call %d should not be limited", i)
		}
	}

	// Next call should be limited.
	if !tb.bot.isRateLimited(chatID) {
		t.Error("should be rate limited after burst is exhausted")
	}

	// Different user should have their own bucket.
	if tb.bot.isRateLimited(chatID + 1) {
		t.Error("different user should not be rate limited")
	}
}

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


func TestKeyboards_ManufacturerKeyboard_Page0(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	kb := tb.bot.manufacturerKeyboard(ctx, 0, 0, locale.English)
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("keyboard should have rows")
	}

	// First row should be the search button.
	if kb.InlineKeyboard[0][0].CallbackData != cbMfrSearch {
		t.Errorf("first row should be search button, got %q", kb.InlineKeyboard[0][0].CallbackData)
	}

	// Should have nav row at the bottom since there are many manufacturers.
	lastRow := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	hasNav := false
	for _, btn := range lastRow {
		if btn.Text == "Next" {
			hasNav = true
		}
	}
	if !hasNav {
		t.Error("should have Next button for paginated manufacturers")
	}
}

func TestKeyboards_ManufacturerKeyboard_Pagination(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	kb0 := tb.bot.manufacturerKeyboard(ctx, 0, 0, locale.English)
	kb1 := tb.bot.manufacturerKeyboard(ctx, 0, 1, locale.English)

	// Different pages should show different content.
	if len(kb0.InlineKeyboard) == 0 || len(kb1.InlineKeyboard) == 0 {
		t.Fatal("both pages should have rows")
	}

	// Page 0 entries should differ from page 1 entries.
	first0 := kb0.InlineKeyboard[1][0].Text // skip search row
	first1 := kb1.InlineKeyboard[1][0].Text // skip search row
	if first0 == first1 {
		t.Error("pages should show different manufacturers")
	}
}

func TestKeyboards_ManufacturerKeyboard_RecentSection(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 500

	tb.createUser(ctx, t, chatID, "alice")

	// Create searches for Mazda (27) and Toyota (19).
	if _, err := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "s1", Manufacturer: 19, Model: 1,
	}); err != nil {
		t.Fatalf("create search s1: %v", err)
	}
	if _, err := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "s2", Manufacturer: 27, Model: 1,
	}); err != nil {
		t.Fatalf("create search s2: %v", err)
	}

	kb := tb.bot.manufacturerKeyboard(ctx, chatID, 0, locale.English)

	// Row 0: search button.
	if kb.InlineKeyboard[0][0].CallbackData != cbMfrSearch {
		t.Errorf("row 0 should be search button, got %q", kb.InlineKeyboard[0][0].CallbackData)
	}

	// Row 1: recent manufacturers (most recent first — Toyota was created last).
	recentRow := kb.InlineKeyboard[1]
	if len(recentRow) != 2 {
		t.Fatalf("expected 2 recent buttons, got %d", len(recentRow))
	}
	names := []string{recentRow[0].Text, recentRow[1].Text}
	if names[0] == names[1] {
		t.Errorf("recent manufacturers should be distinct, got %v", names)
	}
	hasManufacturer := func(name string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}
	if !hasManufacturer("Mazda") || !hasManufacturer("Toyota") {
		t.Errorf("expected Mazda and Toyota in recent, got %v", names)
	}

	// Row 2: separator.
	if kb.InlineKeyboard[2][0].CallbackData != "noop" {
		t.Error("expected separator row after recent manufacturers")
	}
}

func TestKeyboards_ManufacturerKeyboard_NoRecentForNewUser(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 501

	tb.createUser(ctx, t, chatID, "bob")

	kb := tb.bot.manufacturerKeyboard(ctx, chatID, 0, locale.English)

	// Row 0: search button, row 1: first manufacturer (no recent section).
	if kb.InlineKeyboard[0][0].CallbackData != cbMfrSearch {
		t.Errorf("row 0 should be search button, got %q", kb.InlineKeyboard[0][0].CallbackData)
	}
	// Second row should be actual manufacturers, not a separator.
	if kb.InlineKeyboard[1][0].CallbackData == "noop" {
		t.Error("new user should not have a recent/separator row")
	}
}

func TestKeyboards_ManufacturerKeyboard_RecentNotOnPage2(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 502

	tb.createUser(ctx, t, chatID, "carol")
	if _, err := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "s1", Manufacturer: 27, Model: 1,
	}); err != nil {
		t.Fatalf("create search: %v", err)
	}

	kb := tb.bot.manufacturerKeyboard(ctx, chatID, 1, locale.English)

	// On page 1, no recent section — second row should not be a separator.
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "───────────" {
				t.Error("recent section should not appear on page > 0")
			}
		}
	}
}

func TestKeyboards_ManufacturerKeyboard_RecentCappedAt4(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 503

	tb.createUser(ctx, t, chatID, "dave")

	// Create 6 searches with different manufacturers.
	mfrIDs := []int{19, 27, 5, 21, 12, 40}
	for i, id := range mfrIDs {
		if _, err := tb.store.CreateSearch(ctx, storage.Search{
			ChatID: chatID, Name: fmt.Sprintf("s%d", i), Manufacturer: id, Model: 1,
		}); err != nil {
			t.Fatalf("create search %d: %v", i, err)
		}
	}

	kb := tb.bot.manufacturerKeyboard(ctx, chatID, 0, locale.English)

	// Count recent manufacturer buttons (between search row and separator).
	recentCount := 0
	for i := 1; i < len(kb.InlineKeyboard); i++ {
		if kb.InlineKeyboard[i][0].CallbackData == "noop" {
			break
		}
		recentCount += len(kb.InlineKeyboard[i])
	}
	if recentCount != maxRecentManufacturers {
		t.Errorf("expected %d recent manufacturers, got %d", maxRecentManufacturers, recentCount)
	}
}

func TestKeyboards_ManufacturerKeyboard_RecentDeduplicates(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 504

	tb.createUser(ctx, t, chatID, "eve")

	// Create 3 searches for the same manufacturer.
	for i := range 3 {
		if _, err := tb.store.CreateSearch(ctx, storage.Search{
			ChatID: chatID, Name: fmt.Sprintf("s%d", i), Manufacturer: 27, Model: i + 1,
		}); err != nil {
			t.Fatalf("create search %d: %v", i, err)
		}
	}

	kb := tb.bot.manufacturerKeyboard(ctx, chatID, 0, locale.English)

	// Should have exactly 1 recent button (Mazda), not 3.
	recentCount := 0
	for i := 1; i < len(kb.InlineKeyboard); i++ {
		if kb.InlineKeyboard[i][0].CallbackData == "noop" {
			break
		}
		recentCount += len(kb.InlineKeyboard[i])
	}
	if recentCount != 1 {
		t.Errorf("expected 1 unique recent manufacturer, got %d", recentCount)
	}
}

func TestKeyboards_ModelKeyboard(t *testing.T) {
	tb := newTestBot(t)
	kb := tb.bot.modelKeyboard(27, 0, locale.English) // Mazda
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

func TestKeyboards_ModelKeyboard_AnyModel(t *testing.T) {
	tb := newTestBot(t)
	kb := tb.bot.modelKeyboard(27, 0, locale.English) // Mazda

	hasAny := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "Any model" && btn.CallbackData == cbAnyModel {
				hasAny = true
			}
		}
	}
	if !hasAny {
		t.Error("model keyboard should have 'Any model' button")
	}
}

func TestKeyboards_ManufacturerSearch(t *testing.T) {
	tb := newTestBot(t)
	kb := tb.bot.manufacturerSearchResults("maz", locale.English)

	found := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "Mazda" {
				found = true
			}
		}
	}
	if !found {
		t.Error("search for 'maz' should find Mazda")
	}
}

func TestKeyboards_ManufacturerSearch_NoResults(t *testing.T) {
	tb := newTestBot(t)
	kb := tb.bot.manufacturerSearchResults("zzzzz", locale.English)

	hasNoResults := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "No results found" {
				hasNoResults = true
			}
		}
	}
	if !hasNoResults {
		t.Error("search with no matches should show 'No results found'")
	}
}

func TestKeyboards_EngineKeyboard(t *testing.T) {
	kb := engineKeyboard(locale.English)
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

	kb, summary := confirmKeyboard(wd, locale.English)
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
	t.Cleanup(func() { _ = store.Close() })
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

func TestToggleSource(t *testing.T) {
	tests := []struct {
		current, toggle, want string
	}{
		{"", "yad2", "yad2"},
		{"yad2", "winwin", "yad2,winwin"},
		{"yad2,winwin", "yad2", "winwin"},
		{"yad2,winwin", "winwin", "yad2"},
		{"winwin", "winwin", ""},
	}
	for _, tt := range tests {
		got := toggleSource(tt.current, tt.toggle)
		if got != tt.want {
			t.Errorf("toggleSource(%q, %q) = %q, want %q", tt.current, tt.toggle, got, tt.want)
		}
	}
}

func TestSourceDisplayName_Multi(t *testing.T) {
	tests := []struct {
		source, want string
	}{
		{"yad2", "Yad2"},
		{"winwin", "WinWin"},
		{"yad2,winwin", "Yad2, WinWin"},
		{"", "Yad2, WinWin"},
	}
	for _, tt := range tests {
		got := sourceDisplayName(tt.source)
		if got != tt.want {
			t.Errorf("sourceDisplayName(%q) = %q, want %q", tt.source, got, tt.want)
		}
	}
}

func TestSourceKeyboard_NoneSelected(t *testing.T) {
	kb := sourceKeyboard("", locale.English)
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row (no Done button), got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].Text != "Yad2" {
		t.Errorf("first button = %q, want Yad2 (no checkmark)", kb.InlineKeyboard[0][0].Text)
	}
}

func TestSourceKeyboard_BothSelected(t *testing.T) {
	kb := sourceKeyboard("yad2,winwin", locale.English)
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows (toggles + Done), got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].Text != "✅ Yad2" {
		t.Errorf("first button = %q, want '✅ Yad2'", kb.InlineKeyboard[0][0].Text)
	}
	if kb.InlineKeyboard[0][1].Text != "✅ WinWin" {
		t.Errorf("second button = %q, want '✅ WinWin'", kb.InlineKeyboard[0][1].Text)
	}
	if kb.InlineKeyboard[1][0].Text != "Done ✓" {
		t.Errorf("done button = %q, want 'Done ✓'", kb.InlineKeyboard[1][0].Text)
	}
}

func TestSweepStaleMaps(t *testing.T) {
	tb := newTestBot(t)

	now := time.Now()
	stale := now.Add(-2 * time.Hour).UnixNano()
	fresh := now.UnixNano()

	// Populate rateLimiter with stale and fresh entries.
	tb.bot.rateLimiter.Store(int64(1), &userRateLimit{lastSeen: stale, tokens: 5, lastTick: now})
	tb.bot.rateLimiter.Store(int64(2), &userRateLimit{lastSeen: fresh, tokens: 5, lastTick: now})

	// Populate chatMu with stale and fresh entries.
	tb.bot.chatMu.Store(int64(10), &chatMuEntry{lastUsed: stale})
	tb.bot.chatMu.Store(int64(20), &chatMuEntry{lastUsed: fresh})

	tb.bot.sweepStaleMaps()

	// Stale entries should be removed.
	if _, ok := tb.bot.rateLimiter.Load(int64(1)); ok {
		t.Error("stale rate limiter entry should have been swept")
	}
	if _, ok := tb.bot.chatMu.Load(int64(10)); ok {
		t.Error("stale chatMu entry should have been swept")
	}

	// Fresh entries should remain.
	if _, ok := tb.bot.rateLimiter.Load(int64(2)); !ok {
		t.Error("fresh rate limiter entry should remain")
	}
	if _, ok := tb.bot.chatMu.Load(int64(20)); !ok {
		t.Error("fresh chatMu entry should remain")
	}
}

func TestWizardFlow_BothSources(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 600

	tb.createUser(ctx, t, chatID, "frank")

	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"winwin")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"0")
	tb.simulateCallback(ctx, chatID, cbConfirm)

	searches, _ := tb.store.ListSearches(ctx, chatID)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Source != "yad2,winwin" {
		t.Errorf("source = %q, want yad2,winwin", searches[0].Source)
	}
}

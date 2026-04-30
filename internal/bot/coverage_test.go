package bot

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected setup error: %v", err)
	}
}

func newTestBotFull(t *testing.T) *testBot {
	t.Helper()
	store, err := sqlite.New("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	mm := &mockMessenger{}
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))

	b := &Bot{
		msg:          mm,
		users:        store,
		searches:     store,
		listings:     store,
		digests:      store,
		saved:        store,
		hidden:       store,
		dailyDigests: store,
		catalog:      catalog.NewStatic(),
		adminChatID:  999,
		maxSearches:  defaultMaxSearches,
		botUsername:   "test_bot",
		logger:       logger,
	}

	return &testBot{bot: b, msg: mm, store: store}
}

// --- /language command ---

func TestHandleLanguage_ShowsKeyboard(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.simulateCommand(ctx, 100, "/language")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected keyboard for language selection")
	}
}

func TestLanguageSwitch_Hebrew(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.simulateCallback(ctx, 100, "lang:he")

	user, err := tb.store.GetUser(ctx, 100)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Language != "he" {
		t.Errorf("language = %q, want 'he'", user.Language)
	}
}

func TestLanguageSwitch_English(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")
	mustNoErr(t, tb.store.SetUserLanguage(ctx, 100, "he"))

	tb.simulateCallback(ctx, 100, "lang:en")

	user, err := tb.store.GetUser(ctx, 100)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Language != "en" {
		t.Errorf("language = %q, want 'en'", user.Language)
	}
}

// --- /saved and /hidden commands ---

func TestHandleSaved_Empty(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.simulateCommand(ctx, 100, "/saved")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Error("expected message for empty saved list")
	}
}

func TestHandleHidden_Empty(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.simulateCommand(ctx, 100, "/hidden")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Error("expected message for empty hidden list")
	}
}

func TestOnClearHidden(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.HideListing(ctx, 100, "tok1"))
	mustNoErr(t, tb.store.HideListing(ctx, 100, "tok2"))

	tb.simulateCallback(ctx, 100, "hidden_clear")

	count, err := tb.store.CountHidden(ctx, 100)
	if err != nil {
		t.Fatalf("CountHidden: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 hidden after clear, got %d", count)
	}
}

// --- Quick start ---

func TestOnQuickStart_Success(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.simulateCallback(ctx, 100, "quick_start")

	searches, err := tb.store.ListSearches(ctx, 100)
	if err != nil {
		t.Fatalf("ListSearches: %v", err)
	}
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Manufacturer != 19 {
		t.Errorf("expected Toyota (19), got %d", searches[0].Manufacturer)
	}
}

func TestOnQuickStart_AtLimit(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	for i := range 10 {
		if _, err := tb.store.CreateSearch(ctx, storage.Search{
			ChatID: 100, Name: fmt.Sprintf("s%d", i), Source: "yad2",
			Manufacturer: 27, Model: 10332, Active: true,
		}); err != nil {
			t.Fatalf("create search: %v", err)
		}
	}

	tb.simulateCallback(ctx, 100, "quick_start")

	searches, err := tb.store.ListSearches(ctx, 100)
	if err != nil {
		t.Fatalf("ListSearches: %v", err)
	}
	if len(searches) != 10 {
		t.Errorf("should not create beyond limit, got %d searches", len(searches))
	}
}

// --- Watch from callback ---

func TestOnWatchFromCallback(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.simulateCallback(ctx, 100, "watch")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected source selection keyboard")
	}
}

// --- Wizard navigation ---

func TestOnMfrPage(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, "ask_manufacturer", "{}"))

	tb.simulateCallback(ctx, 100, "mfr_pg:0")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected manufacturer keyboard")
	}
}

func TestOnMfrSearch(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, "ask_manufacturer", "{}"))

	tb.simulateCallback(ctx, 100, "mfr_search")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Error("expected search prompt message")
	}
}

func TestOnMdlPage(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, "ask_model", `{"manufacturer":19,"manufacturer_name":"Toyota"}`))

	tb.simulateCallback(ctx, 100, "mdl_pg:0")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected model keyboard")
	}
}

func TestOnMdlSearch(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, "ask_model", `{"manufacturer":19}`))

	tb.simulateCallback(ctx, 100, "mdl_search")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Error("expected model search prompt")
	}
}

func TestHandleManufacturerSearch(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, "search_manufacturer", "{}"))

	tb.simulateText(ctx, 100, "Toyota")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected search results keyboard")
	}
}

func TestHandleModelSearch(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, "search_model", `{"manufacturer":19}`))

	tb.simulateText(ctx, 100, "Corolla")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected model search results keyboard")
	}
}

// --- Daily digest callbacks ---

func TestOnDailyDigestOff(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.SetUserTier(ctx, 100, "premium", time.Now().Add(30*24*time.Hour)))
	mustNoErr(t, tb.store.SetDailyDigest(ctx, 100, true, "09:00"))

	tb.simulateCallback(ctx, 100, "daily_digest:off")

	enabled, _, _, err := tb.store.GetDailyDigest(ctx, 100)
	if err != nil {
		t.Fatalf("GetDailyDigest: %v", err)
	}
	if enabled {
		t.Error("expected daily digest to be disabled")
	}
}

// --- formatInterval ---

func TestFormatInterval_Minutes(t *testing.T) {
	tb := newTestBotFull(t)
	tb.bot.pollInterval = 15 * time.Minute

	got := tb.bot.formatInterval()
	if got != "15 minutes" {
		t.Errorf("formatInterval() = %q, want '15 minutes'", got)
	}
}

func TestFormatInterval_Seconds(t *testing.T) {
	tb := newTestBotFull(t)
	tb.bot.pollInterval = 30 * time.Second

	got := tb.bot.formatInterval()
	if got != "30s" {
		t.Errorf("formatInterval() = %q, want '30s'", got)
	}
}

func TestFormatInterval_Hours(t *testing.T) {
	tb := newTestBotFull(t)
	tb.bot.pollInterval = 2 * time.Hour

	got := tb.bot.formatInterval()
	if got != "2 hours" {
		t.Errorf("formatInterval() = %q, want '2 hours'", got)
	}
}

func TestFormatInterval_OneHour(t *testing.T) {
	tb := newTestBotFull(t)
	tb.bot.pollInterval = 1 * time.Hour

	got := tb.bot.formatInterval()
	if got != "1 hour" {
		t.Errorf("formatInterval() = %q, want '1 hour'", got)
	}
}

func TestFormatInterval_HoursAndMinutes(t *testing.T) {
	tb := newTestBotFull(t)
	tb.bot.pollInterval = 1*time.Hour + 30*time.Minute

	got := tb.bot.formatInterval()
	if got != "1h30m0s" {
		t.Errorf("formatInterval() = %q, want '1h30m0s'", got)
	}
}

// --- SetPollTrigger ---

type mockPollTrigger struct {
	triggered bool
}

func (m *mockPollTrigger) TriggerPoll() { m.triggered = true }

func TestSetPollTrigger(t *testing.T) {
	tb := newTestBotFull(t)
	pt := &mockPollTrigger{}
	tb.bot.SetPollTrigger(pt)
	if tb.bot.pollTrigger == nil {
		t.Error("pollTrigger should be set")
	}
}

// --- handleGrantPremium ---

// --- onSavedPage / onHiddenPage ---

func TestOnSavedPage_WithData(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	for i := range 15 {
		token := fmt.Sprintf("saved-tok-%02d", i)
		mustNoErr(t, tb.store.SaveListing(ctx, storage.ListingRecord{
			Token: token, ChatID: 100, SearchName: "s1",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		}))
		mustNoErr(t, tb.store.SaveBookmark(ctx, 100, token))
	}

	tb.bot.onSavedPage(ctx, 100, cbSavedPage+"0")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected keyboard with pagination")
	}
}

func TestOnSavedPage_InvalidPage(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.bot.onSavedPage(ctx, 100, cbSavedPage+"abc")
}

func TestOnHiddenPage_WithData(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	for i := range 15 {
		token := fmt.Sprintf("hidden-tok-%02d", i)
		mustNoErr(t, tb.store.HideListing(ctx, 100, token))
	}

	tb.bot.onHiddenPage(ctx, 100, cbHiddenPage+"0")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected keyboard with pagination for hidden")
	}
}

func TestOnHiddenPage_InvalidPage(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.bot.onHiddenPage(ctx, 100, cbHiddenPage+"abc")
}

// --- ListingActionKeyboard ---

func TestListingActionKeyboard(t *testing.T) {
	kb := ListingActionKeyboard("test-token", "en")
	if kb == nil {
		t.Fatal("expected non-nil keyboard")
	}
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 1 row with 2 buttons, got %v", kb.InlineKeyboard)
	}
	if kb.InlineKeyboard[0][0].CallbackData != "save:test-token" {
		t.Errorf("save button data = %q", kb.InlineKeyboard[0][0].CallbackData)
	}
	if kb.InlineKeyboard[0][1].CallbackData != "hide:test-token" {
		t.Errorf("hide button data = %q", kb.InlineKeyboard[0][1].CallbackData)
	}
}

// --- isRateLimited ---

func TestIsRateLimited_Exhaustion(t *testing.T) {
	tb := newTestBotFull(t)

	for range rateLimitBurst {
		if tb.bot.isRateLimited(555) {
			t.Error("should not be limited within burst")
		}
	}

	if !tb.bot.isRateLimited(555) {
		t.Error("should be limited after burst exhausted")
	}
}

// --- sweepStaleMaps ---

func TestSweepStaleMaps_NoError(t *testing.T) {
	tb := newTestBotFull(t)
	tb.bot.isRateLimited(555)
	unlock := tb.bot.lockChat(556)
	unlock()

	tb.bot.sweepStaleMaps()
}

// --- maxSearchesForUser ---

func TestMaxSearchesForUser(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	limit := tb.bot.maxSearchesForUser(ctx, 100)
	if limit != defaultMaxSearches {
		t.Errorf("maxSearches = %d, want %d", limit, defaultMaxSearches)
	}
}

// --- saveWizardState / loadWizardData ---

func TestSaveAndLoadWizardState(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	wd := WizardData{
		Source:           "yad2",
		Manufacturer:     19,
		ManufacturerName: "Toyota",
	}
	tb.bot.saveWizardState(ctx, 100, "ask_model", wd)

	loaded := tb.bot.loadWizardData(ctx, 100)
	if loaded.Manufacturer != 19 {
		t.Errorf("manufacturer = %d, want 19", loaded.Manufacturer)
	}
	if loaded.ManufacturerName != "Toyota" {
		t.Errorf("manufacturer_name = %q, want Toyota", loaded.ManufacturerName)
	}
}

func TestLoadWizardData_NoUser(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()

	wd := tb.bot.loadWizardData(ctx, 99999)
	if wd.Manufacturer != 0 {
		t.Errorf("expected empty wizard data for nonexistent user")
	}
}

func TestLoadWizardData_InvalidJSON(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, "ask_model", "not-json"))
	wd := tb.bot.loadWizardData(ctx, 100)
	if wd.Manufacturer != 0 {
		t.Errorf("expected empty wizard data for invalid JSON")
	}
}

// --- getUserLang ---

func TestGetUserLang_Default(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()

	lang := tb.bot.getUserLang(ctx, 99999)
	if lang != "he" {
		t.Errorf("default language = %q, want 'he'", lang)
	}
}

// --- onLegacySourceSelected ---

func TestOnLegacySourceSelected(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	mustNoErr(t, tb.store.UpdateUserState(ctx, 100, StateAskSource, "{}"))

	tb.bot.onLegacySourceSelected(ctx, 100, "yad2")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Error("expected response from source selection")
	}
}

// --- onDailyDigestOn ---

func TestOnDailyDigestOn(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")
	mustNoErr(t, tb.store.SetUserTier(ctx, 100, "premium", time.Now().Add(30*24*time.Hour)))

	tb.simulateCallback(ctx, 100, "daily_digest:on")

	enabled, _, _, err := tb.store.GetDailyDigest(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Error("expected daily digest to be enabled")
	}
}

// --- SetBot ---

func TestSetBot_NilKeepsBotNil(t *testing.T) {
	tb := newTestBotFull(t)

	origMsg := tb.bot.msg

	tb.bot.SetBot(nil)
	if tb.bot.bot != nil {
		t.Error("expected nil bot after SetBot(nil)")
	}
	if tb.bot.msg != origMsg {
		t.Error("messenger should be unchanged when SetBot(nil) — no tgbot to wrap")
	}
}

// --- onHistoryPage invalid ---

func TestOnHistoryPage_InvalidPage(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.bot.onHistoryPage(ctx, 100, cbHistoryPage+"abc")
	msg := tb.msg.last()
	if msg.Text == "" {
		t.Error("expected error message for invalid page")
	}
}

// --- onQuickStart with poll trigger ---

func TestOnQuickStart_TriggersPoll(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	pt := &mockPollTrigger{}
	tb.bot.SetPollTrigger(pt)

	tb.simulateCallback(ctx, 100, "quick_start")

	if !pt.triggered {
		t.Error("expected poll trigger to be called")
	}
}

// --- saved/hidden pagination with multiple pages ---

func TestHandleSaved_WithPagination(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	for i := range 15 {
		token := fmt.Sprintf("sav-pg-%02d", i)
		mustNoErr(t, tb.store.SaveListing(ctx, storage.ListingRecord{
			Token: token, ChatID: 100, SearchName: "s1",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		}))
		mustNoErr(t, tb.store.SaveBookmark(ctx, 100, token))
	}

	tb.simulateCommand(ctx, 100, "/saved")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected keyboard with pagination")
	}
}

func TestHandleHidden_WithPagination(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	for i := range 15 {
		mustNoErr(t, tb.store.HideListing(ctx, 100, fmt.Sprintf("hid-pg-%02d", i)))
	}

	tb.simulateCommand(ctx, 100, "/hidden")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected keyboard with pagination for hidden")
	}
}

// --- onSavedPage page 1 ---

func TestOnSavedPage_Page1(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	for i := range 20 {
		token := fmt.Sprintf("sav-p1-%02d", i)
		mustNoErr(t, tb.store.SaveListing(ctx, storage.ListingRecord{
			Token: token, ChatID: 100, SearchName: "s1",
			Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		}))
		mustNoErr(t, tb.store.SaveBookmark(ctx, 100, token))
	}

	tb.bot.onSavedPage(ctx, 100, cbSavedPage+"1")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected keyboard on page 1")
	}
}

// --- onHiddenPage page 1 ---

func TestOnHiddenPage_Page1(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	for i := range 20 {
		mustNoErr(t, tb.store.HideListing(ctx, 100, fmt.Sprintf("hid-p1-%02d", i)))
	}

	tb.bot.onHiddenPage(ctx, 100, cbHiddenPage+"1")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("expected keyboard on hidden page 1")
	}
}

package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tgbot "github.com/go-telegram/bot"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
)

// --- Markdown Safety Tests (TDD: these catch the "stuck after model select" bug) ---

func TestSend_PlainTextHasNoParseMode(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	tb.bot.send(ctx, 100, "From which year? (e.g. 2018)")

	msg := tb.msg.last()
	if msg.ParseMode != "" {
		t.Errorf("send() should use no parse mode, got %q", msg.ParseMode)
	}
}

func TestSendMarkdown_HasParseMode(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	tb.bot.sendMarkdown(ctx, 100, "Welcome to *CarWatch*!")

	msg := tb.msg.last()
	if msg.ParseMode != "Markdown" {
		t.Errorf("sendMarkdown() should use Markdown parse mode, got %q", msg.ParseMode)
	}
}

func TestWizardPrompts_NoMarkdownParseMode(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")

	// Walk through the wizard until we hit text prompts.
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.msg.reset()

	// Select a manufacturer that has models (Mazda, ID 27).
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.msg.reset()

	// Select a model — this triggers "From which year? (e.g. 2018)".
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Fatal("expected a message after model selection, got nothing")
	}
	if msg.ParseMode != "" {
		t.Errorf("year prompt should be plain text (no parse mode), got %q — message: %q", msg.ParseMode, msg.Text)
	}
	if !strings.Contains(msg.Text, "(e.g.") {
		t.Errorf("expected year prompt with parentheses, got: %q", msg.Text)
	}
}

func TestWizardYearMaxPrompt_NoMarkdownParseMode(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.msg.reset()

	// Enter year min — triggers "Until which year? (e.g. 2024)".
	tb.simulateText(ctx, chatID, "2018")

	msg := tb.msg.last()
	if msg.ParseMode != "" {
		t.Errorf("year max prompt should be plain text, got parseMode=%q — message: %q", msg.ParseMode, msg.Text)
	}
}

func TestWizardPricePrompt_NoMarkdownParseMode(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.msg.reset()

	// Enter year max — triggers "Max price in NIS? (e.g. 150000)".
	tb.simulateText(ctx, chatID, "2024")

	msg := tb.msg.last()
	if msg.ParseMode != "" {
		t.Errorf("price prompt should be plain text, got parseMode=%q — message: %q", msg.ParseMode, msg.Text)
	}
}

// --- Full Wizard Flow Test ---

func TestWizardFlow_EndToEnd(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")

	// Step 1: /watch → source keyboard
	tb.simulateCommand(ctx, chatID, "/watch")
	msg := tb.msg.last()
	if !msg.HasKB || msg.Buttons != 2 {
		t.Fatalf("step 1: expected 2-button keyboard, got buttons=%d hasKB=%v", msg.Buttons, msg.HasKB)
	}
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskSource {
		t.Fatalf("step 1: state=%q, want %q", user.State, StateAskSource)
	}

	// Step 2: toggle source and confirm → manufacturer keyboard
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	msg = tb.msg.last()
	if !msg.HasKB || msg.Buttons < 10 {
		t.Fatalf("step 2: expected manufacturer keyboard with many buttons, got %d", msg.Buttons)
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskManufacturer {
		t.Fatalf("step 2: state=%q, want %q", user.State, StateAskManufacturer)
	}

	// Step 3: select manufacturer → model keyboard
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27") // Mazda
	msg = tb.msg.last()
	if !msg.HasKB {
		t.Fatal("step 3: expected model keyboard")
	}
	if !strings.Contains(msg.Text, "Mazda") {
		t.Errorf("step 3: text should mention Mazda, got %q", msg.Text)
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskModel {
		t.Fatalf("step 3: state=%q, want %q", user.State, StateAskModel)
	}

	// Step 4: select model → year min prompt
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332") // Mazda 3
	msg = tb.msg.last()
	if msg.Text == "" {
		t.Fatal("step 4: expected year prompt")
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskYearMin {
		t.Fatalf("step 4: state=%q, want %q", user.State, StateAskYearMin)
	}

	// Step 5: enter year min → year max prompt
	tb.simulateText(ctx, chatID, "2018")
	msg = tb.msg.last()
	if !strings.Contains(msg.Text, "which year") {
		t.Errorf("step 5: expected year max prompt, got %q", msg.Text)
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskYearMax {
		t.Fatalf("step 5: state=%q, want %q", user.State, StateAskYearMax)
	}

	// Step 6: enter year max → price prompt
	tb.simulateText(ctx, chatID, "2024")
	msg = tb.msg.last()
	if !strings.Contains(msg.Text, "price") {
		t.Errorf("step 6: expected price prompt, got %q", msg.Text)
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskPriceMax {
		t.Fatalf("step 6: state=%q, want %q", user.State, StateAskPriceMax)
	}

	// Step 7: enter price → engine keyboard
	tb.simulateText(ctx, chatID, "150000")
	msg = tb.msg.last()
	if !msg.HasKB {
		t.Fatal("step 7: expected engine keyboard")
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskEngine {
		t.Fatalf("step 7: state=%q, want %q", user.State, StateAskEngine)
	}

	// Step 8: select engine → max km keyboard
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"2000")
	msg = tb.msg.last()
	if !msg.HasKB || !strings.Contains(msg.Text, "kilometers") {
		t.Fatalf("step 8: expected max km keyboard, got %q", msg.Text)
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskMaxKm {
		t.Fatalf("step 8: state=%q, want %q", user.State, StateAskMaxKm)
	}

	// Step 9: select max km → max hand keyboard
	tb.simulateCallback(ctx, chatID, cbPrefixMaxKm+"100000")
	msg = tb.msg.last()
	if !msg.HasKB || !strings.Contains(msg.Text, "hand") {
		t.Fatalf("step 9: expected max hand keyboard, got %q", msg.Text)
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskMaxHand {
		t.Fatalf("step 9: state=%q, want %q", user.State, StateAskMaxHand)
	}

	// Step 10: select max hand → keywords prompt
	tb.simulateCallback(ctx, chatID, cbPrefixMaxHand+"3")
	msg = tb.msg.last()
	if !msg.HasKB {
		t.Fatal("step 10: expected keywords prompt with skip button")
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskKeywords {
		t.Fatalf("step 10: state=%q, want %q", user.State, StateAskKeywords)
	}

	// Step 11: skip keywords → exclude-keys prompt
	tb.simulateCallback(ctx, chatID, cbSkipKeywords)
	msg = tb.msg.last()
	if !msg.HasKB {
		t.Fatal("step 11: expected exclude-keys prompt with skip button")
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateAskExcludeKeys {
		t.Fatalf("step 11: state=%q, want %q", user.State, StateAskExcludeKeys)
	}

	// Step 12: skip exclude-keys → confirm
	tb.simulateCallback(ctx, chatID, cbSkipExcludeKeys)
	msg = tb.msg.last()
	if !msg.HasKB || !strings.Contains(msg.Text, "Mazda") {
		t.Fatalf("step 12: expected confirm summary with Mazda, got %q", msg.Text)
	}
	if !strings.Contains(msg.Text, "150,000") {
		t.Errorf("step 12: confirm should show formatted price 150,000, got %q", msg.Text)
	}
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateConfirm {
		t.Fatalf("step 12: state=%q, want %q", user.State, StateConfirm)
	}

	// Step 13: confirm → search created, state back to idle
	tb.simulateCallback(ctx, chatID, cbConfirm)
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateIdle {
		t.Fatalf("step 11: state=%q, want %q", user.State, StateIdle)
	}

	// Verify search was saved
	searches, err := tb.store.ListSearches(ctx, chatID)
	if err != nil {
		t.Fatalf("list searches: %v", err)
	}
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	s := searches[0]
	if s.Manufacturer != 27 {
		t.Errorf("manufacturer=%d, want 27", s.Manufacturer)
	}
	if s.Model != 10332 {
		t.Errorf("model=%d, want 10332", s.Model)
	}
	if s.YearMin != 2018 || s.YearMax != 2024 {
		t.Errorf("years=%d-%d, want 2018-2024", s.YearMin, s.YearMax)
	}
	if s.PriceMax != 150000 {
		t.Errorf("price_max=%d, want 150000", s.PriceMax)
	}
	if s.EngineMinCC != 2000 {
		t.Errorf("engine_min_cc=%d, want 2000", s.EngineMinCC)
	}
	if s.MaxKm != 100000 {
		t.Errorf("max_km=%d, want 100000", s.MaxKm)
	}
	if s.MaxHand != 3 {
		t.Errorf("max_hand=%d, want 3", s.MaxHand)
	}
	if s.Source != "yad2" {
		t.Errorf("source=%q, want yad2", s.Source)
	}

	// The confirmation message should mention the search ID
	msg = tb.msg.last()
	if !strings.Contains(msg.Text, "#") {
		t.Errorf("confirm message should contain search ID, got %q", msg.Text)
	}
}

// --- Error Handling Edge Cases ---

func TestCallback_InvalidManufacturerID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"notanumber")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Fatal("expected error message for invalid manufacturer ID")
	}
	if !strings.Contains(msg.Text, "wrong") && !strings.Contains(msg.Text, "cancel") {
		t.Errorf("expected user-friendly error, got %q", msg.Text)
	}
}

func TestCallback_InvalidModelID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbPrefixModel+"bad")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Fatal("expected error message for invalid model ID")
	}
}

func TestCallback_InvalidEngineCC(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"xyz")

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Fatal("expected error message for invalid engine CC")
	}
}

func TestManufacturerWithNoModels(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.msg.reset()

	// Select a real manufacturer that exists in the catalog but has no predefined models.
	// Should show model keyboard with "Any model" button instead of an error.
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"35") // Subaru

	msg := tb.msg.last()
	if msg.Text == "" {
		t.Fatal("expected model selection message")
	}
	if !msg.HasKB {
		t.Error("expected keyboard with 'Any model' button")
	}
}

func TestYearMin_InvalidInput(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.msg.reset()

	tb.simulateText(ctx, chatID, "abc")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "valid year") {
		t.Errorf("expected validation error, got %q", msg.Text)
	}

	// State should NOT have advanced.
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskYearMin {
		t.Errorf("state should stay at %q after invalid input, got %q", StateAskYearMin, user.State)
	}
}

func TestYearMax_LessThanMin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2020")
	tb.msg.reset()

	tb.simulateText(ctx, chatID, "2019") // less than 2020

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, ">=") {
		t.Errorf("expected '>=' validation, got %q", msg.Text)
	}
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskYearMax {
		t.Errorf("state should stay at %q, got %q", StateAskYearMax, user.State)
	}
}

func TestPrice_OutOfRange(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.msg.reset()

	tb.simulateText(ctx, chatID, "500") // too low

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "valid price") {
		t.Errorf("expected price validation error, got %q", msg.Text)
	}
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskPriceMax {
		t.Errorf("state should stay at %q, got %q", StateAskPriceMax, user.State)
	}
}

func TestWatch_AtMaxSearches(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")

	for i := range 10 {
		_, _ = tb.store.CreateSearch(ctx, newFakeSearch(chatID, i+1))
	}
	tb.msg.reset()

	tb.simulateCommand(ctx, chatID, "/watch")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "max") || !strings.Contains(msg.Text, "10") {
		t.Errorf("expected max-searches warning, got %q", msg.Text)
	}
}

func TestCancel_ResetsState(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")

	// User is mid-wizard at StateAskModel
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskModel {
		t.Fatalf("expected state %q, got %q", StateAskModel, user.State)
	}

	tb.simulateCommand(ctx, chatID, "/cancel")
	user, _ = tb.store.GetUser(ctx, chatID)
	if user.State != StateIdle {
		t.Errorf("after /cancel, state=%q, want %q", user.State, StateIdle)
	}
}

func TestCancelCallback_ResetsState(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"2000")

	// At confirm step — click cancel
	tb.simulateCallback(ctx, chatID, cbCancel)

	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateIdle {
		t.Errorf("after cancel callback, state=%q, want %q", user.State, StateIdle)
	}

	searches, _ := tb.store.ListSearches(ctx, chatID)
	if len(searches) != 0 {
		t.Errorf("no search should be saved after cancel, got %d", len(searches))
	}
}

func TestEdit_RestartsWizard(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"2000")

	// At confirm step — click edit
	tb.simulateCallback(ctx, chatID, cbEdit)

	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskSource {
		t.Errorf("after edit, state=%q, want %q", user.State, StateAskSource)
	}
}

func TestUnexpectedText_InIdleState(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.msg.reset()

	tb.simulateText(ctx, chatID, "hello bot")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "didn't understand") {
		t.Errorf("expected 'didn't understand' message, got %q", msg.Text)
	}
}

func TestDeleteSearch_ViaCallback(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, newFakeSearch(chatID, 1))
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbDeleteSearch+"1")

	searches, _ := tb.store.ListSearches(ctx, chatID)
	_ = id
	if len(searches) != 0 {
		t.Errorf("expected 0 searches after delete, got %d", len(searches))
	}
}

// --- Catalog Integrity Tests ---

func TestCatalog_ManufacturersWithoutModels_HaveAnyModelFallback(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"yad2")
	tb.simulateCallback(ctx, chatID, cbSourceDone)

	// Pick a manufacturer that has no models in the static catalog (e.g., Subaru=35).
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"35")
	msg := tb.msg.last()
	if !msg.HasKB {
		t.Fatal("should show keyboard even for manufacturer with no models")
	}

	// Verify the keyboard has an "Any model" button.
	kb := tb.bot.modelKeyboard(35, 0, locale.English)
	hasAnyBtn := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "Any model" && btn.CallbackData == cbAnyModel {
				hasAnyBtn = true
			}
		}
	}
	if !hasAnyBtn {
		t.Error("model keyboard should contain 'Any model' button")
	}

	// Selecting "Any model" should advance the wizard.
	tb.simulateCallback(ctx, chatID, cbAnyModel)
	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskYearMin {
		t.Errorf("after selecting Any model, state should be %q, got %q", StateAskYearMin, user.State)
	}
}

func TestCatalog_NoEmptyNames(t *testing.T) {
	cat := catalog.NewStatic()
	for _, m := range cat.Manufacturers() {
		if m.Name == "" {
			t.Errorf("manufacturer ID=%d has empty name", m.ID)
		}
		for _, mdl := range cat.Models(m.ID) {
			if mdl.Name == "" {
				t.Errorf("model ID=%d under manufacturer %q has empty name", mdl.ID, m.Name)
			}
		}
	}
}

func TestCatalog_NoDuplicateIDs(t *testing.T) {
	cat := catalog.NewStatic()
	seen := make(map[int]string)
	for _, m := range cat.Manufacturers() {
		if prev, ok := seen[m.ID]; ok {
			t.Errorf("duplicate manufacturer ID=%d: %q and %q", m.ID, prev, m.Name)
		}
		seen[m.ID] = m.Name
	}

	for _, m := range cat.Manufacturers() {
		modelSeen := make(map[int]string)
		for _, mdl := range cat.Models(m.ID) {
			if prev, ok := modelSeen[mdl.ID]; ok {
				t.Errorf("duplicate model ID=%d under %q: %q and %q", mdl.ID, m.Name, prev, mdl.Name)
			}
			modelSeen[mdl.ID] = mdl.Name
		}
	}
}

func TestCatalog_ManufacturerNameLookup(t *testing.T) {
	cat := catalog.NewStatic()
	for _, m := range cat.Manufacturers() {
		name := cat.ManufacturerName(m.ID)
		if name != m.Name {
			t.Errorf("ManufacturerName(%d) = %q, want %q", m.ID, name, m.Name)
		}
	}
	if name := cat.ManufacturerName(99999); name != "Unknown" {
		t.Errorf("ManufacturerName(99999) = %q, want 'Unknown'", name)
	}
}

func TestCatalog_ModelNameLookup(t *testing.T) {
	cat := catalog.NewStatic()
	for _, m := range cat.Manufacturers() {
		for _, mdl := range cat.Models(m.ID) {
			name := cat.ModelName(m.ID, mdl.ID)
			if name != mdl.Name {
				t.Errorf("ModelName(%d, %d) = %q, want %q", m.ID, mdl.ID, name, mdl.Name)
			}
		}
	}
	if name := cat.ModelName(27, 99999); name != "Unknown" {
		t.Errorf("ModelName(27, 99999) = %q, want 'Unknown'", name)
	}
}

// --- Command Handler Tests ---

func TestHandleList_Empty(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/list")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "no active searches") {
		t.Errorf("expected empty list message, got %q", msg.Text)
	}
}

func TestHandleList_WithSearches(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	_, _ = tb.store.CreateSearch(ctx, newFakeSearch(chatID, 27))
	tb.msg.reset()

	tb.simulateCommand(ctx, chatID, "/list")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Error("list should include delete buttons")
	}
	if !strings.Contains(msg.Text, "1") {
		t.Errorf("list should show search count, got %q", msg.Text)
	}
}

func TestHandleStop_NoArg(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")

	update := fakeMessage(chatID, "/stop")
	var nilBot *tgbot.Bot
	tb.bot.handleStop(ctx, nilBot, update)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Usage") {
		t.Errorf("expected usage message, got %q", msg.Text)
	}
}

func TestHandleStop_WithID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, newFakeSearch(chatID, 27))
	tb.msg.reset()

	update := fakeMessage(chatID, fmt.Sprintf("/stop %d", id))
	var nilBot *tgbot.Bot
	tb.bot.handleStop(ctx, nilBot, update)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "deleted") {
		t.Errorf("expected deleted message, got %q", msg.Text)
	}

	searches, _ := tb.store.ListSearches(ctx, chatID)
	if len(searches) != 0 {
		t.Errorf("expected 0 searches after delete, got %d", len(searches))
	}
}

func TestHandleHelp(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/help")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "/watch") || !strings.Contains(msg.Text, "/list") {
		t.Errorf("help message should list commands, got %q", msg.Text)
	}
	if msg.ParseMode != "Markdown" {
		t.Errorf("help should use Markdown, got %q", msg.ParseMode)
	}
}

func TestHandleSettings(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/settings")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "0/1") {
		t.Errorf("settings should show 0/1 (free tier limit), got %q", msg.Text)
	}
}

func TestHandleStart(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.simulateCommand(ctx, chatID, "/start")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "CarWatch") {
		t.Errorf("start message should mention CarWatch, got %q", msg.Text)
	}
}

func TestHandlePause(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, newFakeSearch(chatID, 27))
	tb.msg.reset()

	update := fakeMessage(chatID, fmt.Sprintf("/pause %d", id))
	var nilBot *tgbot.Bot
	tb.bot.handlePause(ctx, nilBot, update)

	s, _ := tb.store.GetSearch(ctx, id)
	if s.Active {
		t.Error("search should be paused")
	}

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "paused") {
		t.Errorf("expected paused message, got %q", msg.Text)
	}
}

func TestHandleResume(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, newFakeSearch(chatID, 27))
	if err := tb.store.SetSearchActive(ctx, id, chatID, false); err != nil {
		t.Fatalf("set search inactive: %v", err)
	}
	tb.msg.reset()

	update := fakeMessage(chatID, fmt.Sprintf("/resume %d", id))
	var nilBot *tgbot.Bot
	tb.bot.handleResume(ctx, nilBot, update)

	s, _ := tb.store.GetSearch(ctx, id)
	if !s.Active {
		t.Error("search should be active after resume")
	}
}

func TestHandleStats_NonAdmin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100 // not admin (999)

	tb.createUser(ctx, t, chatID, "alice")
	update := fakeMessage(chatID, "/stats")
	var nilBot *tgbot.Bot
	tb.bot.handleStats(ctx, nilBot, update)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "didn't understand") {
		t.Errorf("non-admin should get rejected, got %q", msg.Text)
	}
}

func TestHandleStats_Admin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 999 // admin

	tb.createUser(ctx, t, chatID, "admin")
	update := fakeMessage(chatID, "/stats")
	var nilBot *tgbot.Bot
	tb.bot.handleStats(ctx, nilBot, update)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Stats") {
		t.Errorf("admin should see stats, got %q", msg.Text)
	}
}

// --- Helpers ---

func newFakeSearch(chatID int64, mfr int) storage.Search {
	return storage.Search{
		ChatID:       chatID,
		Name:         "test",
		Manufacturer: mfr,
		Model:        1,
		YearMin:      2020,
		YearMax:      2024,
		PriceMax:     100000,
	}
}

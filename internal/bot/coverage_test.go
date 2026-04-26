package bot

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

func newTestBotFull(t *testing.T) *testBot {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

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
		maxSearches:  3,
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

	user, _ := tb.store.GetUser(ctx, 100)
	if user.Language != "he" {
		t.Errorf("language = %q, want 'he'", user.Language)
	}
}

func TestLanguageSwitch_English(t *testing.T) {
	tb := newTestBotFull(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")
	_ = tb.store.SetUserLanguage(ctx, 100, "he")

	tb.simulateCallback(ctx, 100, "lang:en")

	user, _ := tb.store.GetUser(ctx, 100)
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

	_ = tb.store.HideListing(ctx, 100, "tok1")
	_ = tb.store.HideListing(ctx, 100, "tok2")

	tb.simulateCallback(ctx, 100, "hidden_clear")

	count, _ := tb.store.CountHidden(ctx, 100)
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

	searches, _ := tb.store.ListSearches(ctx, 100)
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

	for i := range 3 {
		if _, err := tb.store.CreateSearch(ctx, storage.Search{
			ChatID: 100, Name: "s" + string(rune('0'+i)), Source: "yad2",
			Manufacturer: 27, Model: 10332, Active: true,
		}); err != nil {
			t.Fatalf("create search: %v", err)
		}
	}

	tb.simulateCallback(ctx, 100, "quick_start")

	searches, _ := tb.store.ListSearches(ctx, 100)
	if len(searches) != 3 {
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

	_ = tb.store.UpdateUserState(ctx, 100, "ask_manufacturer", "{}")

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

	_ = tb.store.UpdateUserState(ctx, 100, "ask_manufacturer", "{}")

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

	_ = tb.store.UpdateUserState(ctx, 100, "ask_model", `{"manufacturer":19,"manufacturer_name":"Toyota"}`)

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

	_ = tb.store.UpdateUserState(ctx, 100, "ask_model", `{"manufacturer":19}`)

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

	_ = tb.store.UpdateUserState(ctx, 100, "search_manufacturer", "{}")

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

	_ = tb.store.UpdateUserState(ctx, 100, "search_model", `{"manufacturer":19}`)

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

	_ = tb.store.SetUserTier(ctx, 100, "premium", time.Now().Add(30*24*time.Hour))
	_ = tb.store.SetDailyDigest(ctx, 100, true, "09:00")

	tb.simulateCallback(ctx, 100, "daily_digest:off")

	enabled, _, _, _ := tb.store.GetDailyDigest(ctx, 100)
	if enabled {
		t.Error("expected daily digest to be disabled")
	}
}

// --- formatInterval ---

func TestFormatInterval(t *testing.T) {
	tb := newTestBotFull(t)
	tb.bot.pollInterval = 15 * 60_000_000_000

	got := tb.bot.formatInterval()
	if !strings.Contains(got, "15") {
		t.Errorf("formatInterval() = %q, want it to contain '15'", got)
	}
}

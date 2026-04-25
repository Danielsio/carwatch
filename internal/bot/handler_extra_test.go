package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
)

func TestHandlePause_NoArg(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/pause")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Usage") {
		t.Errorf("expected usage message, got %q", msg.Text)
	}
}

func TestHandlePause_InvalidID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/pause abc")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Invalid") {
		t.Errorf("expected invalid ID message, got %q", msg.Text)
	}
}

func TestHandlePause_NonexistentSearch(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/pause 999")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", msg.Text)
	}
}

func TestHandlePause_AlreadyPaused(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, newFakeSearch(chatID, 27))
	_ = tb.store.SetSearchActive(ctx, id, chatID, false)
	tb.msg.reset()

	tb.simulateCommand(ctx, chatID, fmt.Sprintf("/pause %d", id))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "already paused") {
		t.Errorf("expected 'already paused' message, got %q", msg.Text)
	}
}

func TestHandlePause_OtherUsersSearch(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	tb.createUser(ctx, t, 100, "alice")
	tb.createUser(ctx, t, 200, "bob")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: 200, Name: "bob-search", Manufacturer: 27, Model: 10332,
	})
	tb.msg.reset()

	tb.simulateCommand(ctx, 100, fmt.Sprintf("/pause %d", id))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("should not allow pausing another user's search, got %q", msg.Text)
	}
}

func TestHandleResume_NoArg(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/resume")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Usage") {
		t.Errorf("expected usage message, got %q", msg.Text)
	}
}

func TestHandleResume_InvalidID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/resume abc")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Invalid") {
		t.Errorf("expected invalid ID message, got %q", msg.Text)
	}
}

func TestHandleResume_NonexistentSearch(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/resume 999")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", msg.Text)
	}
}

func TestHandleResume_AlreadyActive(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, newFakeSearch(chatID, 27))
	tb.msg.reset()

	tb.simulateCommand(ctx, chatID, fmt.Sprintf("/resume %d", id))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "already active") {
		t.Errorf("expected 'already active' message, got %q", msg.Text)
	}
}

func TestHandleStop_InvalidID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")

	update := fakeMessage(chatID, "/stop abc")
	var nilBot *tgbot.Bot
	tb.bot.handleStop(ctx, nilBot, update)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Invalid") {
		t.Errorf("expected invalid ID message, got %q", msg.Text)
	}
}

func TestHandleStats_AdminWithHealth(t *testing.T) {
	tb := newTestBot(t)
	h := health.New()
	h.RecordSuccess()
	h.RecordListingsFound(5)
	h.RecordNotificationSent()
	tb.bot.health = h
	ctx := context.Background()
	const chatID int64 = 999

	tb.createUser(ctx, t, chatID, "admin")

	update := fakeMessage(chatID, "/stats")
	var nilBot *tgbot.Bot
	tb.bot.handleStats(ctx, nilBot, update)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Health") {
		t.Errorf("admin stats should include health section, got %q", msg.Text)
	}
}

func TestHandleCallback_NilCallbackQuery(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	update := &tgmodels.Update{CallbackQuery: nil}
	var nilBot *tgbot.Bot
	tb.bot.handleCallback(ctx, nilBot, update)
}

func TestDefaultHandler_NilMessage(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	update := &tgmodels.Update{Message: nil}
	var nilBot *tgbot.Bot
	tb.bot.handleDefault(ctx, nilBot, update)
}

func TestDefaultHandler_UnknownUser(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 999999

	tb.simulateText(ctx, chatID, "hello")
}

func TestDefaultHandler_SlashCommandInIdleState(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.msg.reset()

	tb.simulateText(ctx, chatID, "/unknowncommand")

	if len(tb.msg.messages) != 0 {
		t.Errorf("slash commands in idle should be silently ignored by default handler, got %d messages", len(tb.msg.messages))
	}
}

func TestSourceDisplayName(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"yad2", "Yad2"},
		{"winwin", "WinWin"},
		{"", "Yad2, WinWin"},
		{"unknown", "Yad2, WinWin"},
	}
	for _, tt := range tests {
		got := sourceDisplayName(tt.source)
		if got != tt.want {
			t.Errorf("sourceDisplayName(%q) = %q, want %q", tt.source, got, tt.want)
		}
	}
}

func TestConfirmKeyboard_WithZeroEngine(t *testing.T) {
	wd := WizardData{
		Source:           "yad2",
		ManufacturerName: "Toyota",
		ModelName:        "Corolla",
		YearMin:          2020,
		YearMax:          2024,
		PriceMax:         200000,
		EngineMinCC:      0,
	}

	_, summary := confirmKeyboard(wd, locale.English)
	if !strings.Contains(summary, "Any") {
		t.Errorf("engine should show 'Any' when EngineMinCC is 0, got %q", summary)
	}
}

func TestConfirmKeyboard_EmptySource(t *testing.T) {
	wd := WizardData{
		ManufacturerName: "Mazda",
		ModelName:        "3",
		YearMin:          2020,
		YearMax:          2024,
		PriceMax:         100000,
	}

	_, summary := confirmKeyboard(wd, locale.English)
	if !strings.Contains(summary, "Yad2") {
		t.Errorf("empty source should default to Yad2, got %q", summary)
	}
}


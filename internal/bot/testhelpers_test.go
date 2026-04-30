package bot

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

type sentMessage struct {
	ChatID    int64
	Text      string
	ParseMode string
	HasKB     bool
	Buttons   int
}

type mockMessenger struct {
	mu       sync.Mutex
	messages []sentMessage
}

func (m *mockMessenger) SendMessage(_ context.Context, chatID int64, text string, parseMode string, kb *tgmodels.InlineKeyboardMarkup) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := sentMessage{ChatID: chatID, Text: text, ParseMode: parseMode, HasKB: kb != nil}
	if kb != nil {
		for _, row := range kb.InlineKeyboard {
			msg.Buttons += len(row)
		}
	}
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockMessenger) SendPhoto(_ context.Context, chatID int64, _ string, caption string, parseMode string, kb *tgmodels.InlineKeyboardMarkup) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := sentMessage{ChatID: chatID, Text: caption, ParseMode: parseMode, HasKB: kb != nil}
	if kb != nil {
		for _, row := range kb.InlineKeyboard {
			msg.Buttons += len(row)
		}
	}
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockMessenger) AnswerCallback(_ context.Context, _ string) error {
	return nil
}

func (m *mockMessenger) last() sentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) == 0 {
		return sentMessage{}
	}
	return m.messages[len(m.messages)-1]
}

func (m *mockMessenger) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

type testBot struct {
	bot   *Bot
	msg   *mockMessenger
	store *sqlite.Store
}

func newTestBot(t *testing.T) *testBot {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	mm := &mockMessenger{}
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))

	b := &Bot{
		msg:         mm,
		users:       store,
		searches:    store,
		listings:    store,
		catalog:     catalog.NewStatic(),
		adminChatID: 999,
		maxSearches: 3,
		botUsername:  "test_bot",
		logger:      logger,
	}

	return &testBot{bot: b, msg: mm, store: store}
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func fakeMessage(chatID int64, text string) *tgmodels.Update {
	return &tgmodels.Update{
		Message: &tgmodels.Message{
			Chat: tgmodels.Chat{ID: chatID},
			From: &tgmodels.User{Username: "testuser"},
			Text: text,
		},
	}
}

func fakeCallback(chatID int64, data string) *tgmodels.Update {
	return &tgmodels.Update{
		CallbackQuery: &tgmodels.CallbackQuery{
			ID:   "cb-1",
			Data: data,
			From: tgmodels.User{Username: "testuser"},
			Message: tgmodels.MaybeInaccessibleMessage{
				Message: &tgmodels.Message{
					Chat: tgmodels.Chat{ID: chatID},
				},
			},
		},
	}
}

func newTestBotWithDigests(t *testing.T) *testBot {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	mm := &mockMessenger{}
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))

	b := &Bot{
		msg:         mm,
		users:       store,
		searches:    store,
		listings:    store,
		digests:     store,
		catalog:     catalog.NewStatic(),
		adminChatID: 999,
		maxSearches: 3,
		botUsername:  "test_bot",
		logger:      logger,
	}

	return &testBot{bot: b, msg: mm, store: store}
}

// createUser is a test helper that creates a user with English as the default language.
// Use this in tests that assert English message text.
func (tb *testBot) createUser(ctx context.Context, t *testing.T, chatID int64, username string) {
	t.Helper()
	if err := tb.store.UpsertUser(ctx, chatID, username); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if err := tb.store.SetUserLanguage(ctx, chatID, "en"); err != nil {
		t.Fatalf("set user language: %v", err)
	}
}

func (tb *testBot) simulateCommand(ctx context.Context, chatID int64, text string) {
	update := fakeMessage(chatID, text)
	var nilBot *tgbot.Bot

	switch {
	case text == "/watch":
		tb.bot.handleWatch(ctx, nilBot, update)
	case text == "/list":
		tb.bot.handleList(ctx, nilBot, update)
	case text == "/cancel":
		tb.bot.handleCancel(ctx, nilBot, update)
	case text == "/help":
		tb.bot.handleHelp(ctx, nilBot, update)
	case text == "/settings":
		tb.bot.handleSettings(ctx, nilBot, update)
	case text == "/history":
		tb.bot.handleHistory(ctx, nilBot, update)
	case text == "/digest":
		tb.bot.handleDigest(ctx, nilBot, update)
	case text == "/upgrade":
		tb.bot.handleUpgrade(ctx, nilBot, update)
	case strings.HasPrefix(text, "/start"):
		tb.bot.handleStart(ctx, nilBot, update)
	case strings.HasPrefix(text, "/stop"):
		tb.bot.handleStop(ctx, nilBot, update)
	case strings.HasPrefix(text, "/pause"):
		tb.bot.handlePause(ctx, nilBot, update)
	case strings.HasPrefix(text, "/resume"):
		tb.bot.handleResume(ctx, nilBot, update)
	case strings.HasPrefix(text, "/share"):
		tb.bot.handleShare(ctx, nilBot, update)
	case strings.HasPrefix(text, "/edit"):
		tb.bot.handleEdit(ctx, nilBot, update)
	case text == "/language":
		tb.bot.handleLanguage(ctx, nilBot, update)
	case text == "/saved":
		tb.bot.handleSaved(ctx, nilBot, update)
	case text == "/hidden":
		tb.bot.handleHidden(ctx, nilBot, update)
	default:
		tb.bot.handleDefault(ctx, nilBot, update)
	}
}

func (tb *testBot) simulateCallback(ctx context.Context, chatID int64, data string) {
	update := fakeCallback(chatID, data)
	var nilBot *tgbot.Bot
	tb.bot.handleCallback(ctx, nilBot, update)
}

func (tb *testBot) simulateText(ctx context.Context, chatID int64, text string) {
	update := fakeMessage(chatID, text)
	var nilBot *tgbot.Bot
	tb.bot.handleDefault(ctx, nilBot, update)
}

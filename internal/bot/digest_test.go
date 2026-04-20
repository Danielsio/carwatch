package bot

import (
	"context"
	"strings"
	"testing"
)

func TestHandleDigest_NoDigestStore(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/digest")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not available") {
		t.Errorf("expected 'not available' message when digest store is nil, got %q", msg.Text)
	}
}

func TestHandleDigest_InstantMode(t *testing.T) {
	tb := newTestBotWithDigests(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/digest")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "instant") {
		t.Errorf("expected instant mode message, got %q", msg.Text)
	}
	if !msg.HasKB {
		t.Error("expected keyboard with toggle button")
	}
}

func TestHandleDigest_DigestMode(t *testing.T) {
	tb := newTestBotWithDigests(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	_ = tb.store.SetDigestMode(ctx, chatID, "digest", "6h")

	tb.simulateCommand(ctx, chatID, "/digest")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "digest") {
		t.Errorf("expected digest mode message, got %q", msg.Text)
	}
	if !msg.HasKB {
		t.Error("expected keyboard with toggle button")
	}
}

func TestDigestOn_Callback(t *testing.T) {
	tb := newTestBotWithDigests(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCallback(ctx, chatID, cbDigestOn)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "digest") {
		t.Errorf("expected digest confirmation, got %q", msg.Text)
	}

	mode, _, err := tb.store.GetDigestMode(ctx, chatID)
	if err != nil {
		t.Fatalf("get digest mode: %v", err)
	}
	if mode != "digest" {
		t.Errorf("mode = %q, want 'digest'", mode)
	}
}

func TestDigestOff_Callback(t *testing.T) {
	tb := newTestBotWithDigests(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	_ = tb.store.SetDigestMode(ctx, chatID, "digest", "6h")

	tb.simulateCallback(ctx, chatID, cbDigestOff)

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "instant") {
		t.Errorf("expected instant confirmation, got %q", msg.Text)
	}

	mode, _, err := tb.store.GetDigestMode(ctx, chatID)
	if err != nil {
		t.Fatalf("get digest mode: %v", err)
	}
	if mode != "instant" {
		t.Errorf("mode = %q, want 'instant'", mode)
	}
}

func TestDigestOn_NilStore(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbDigestOn)
	tb.simulateCallback(ctx, chatID, cbDigestOff)

	if len(tb.msg.messages) != 0 {
		t.Error("nil digest store should silently ignore digest callbacks")
	}
}

package bot

import (
	"context"
	"testing"
)

func TestIsLowerHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc123", true},
		{"0123456789abcdef", true},
		{"", true},
		{"ABC", false},
		{"xyz", false},
		{"0x123", false},
		{"12 34", false},
	}
	for _, tt := range tests {
		if got := isLowerHex(tt.input); got != tt.want {
			t.Errorf("isLowerHex(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func setupWizardAtPriceMin(t *testing.T, tb *testBot, chatID int64, priceMax int) {
	t.Helper()
	ctx := context.Background()
	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	wd := tb.bot.loadWizardData(ctx, chatID)
	wd.PriceMax = priceMax
	tb.bot.saveWizardState(ctx, chatID, StateAskPriceMin, wd)
	tb.msg.reset()
}

func TestHandlePriceMin_ValidInput(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	setupWizardAtPriceMin(t, tb, 100, 200000)

	tb.bot.handlePriceMin(ctx, 100, "50000")

	last := tb.msg.last()
	if last.ChatID != 100 {
		t.Errorf("expected message to chat 100, got %d", last.ChatID)
	}
	if !last.HasKB {
		t.Error("expected gearbox keyboard after valid price min")
	}
}

func TestHandlePriceMin_Skip(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	setupWizardAtPriceMin(t, tb, 100, 200000)

	tb.bot.handlePriceMin(ctx, 100, "0")

	last := tb.msg.last()
	if last.ChatID != 100 {
		t.Errorf("expected message to chat 100, got %d", last.ChatID)
	}
	if !last.HasKB {
		t.Error("expected gearbox keyboard after skip")
	}
}

func TestHandlePriceMin_InvalidInput(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	setupWizardAtPriceMin(t, tb, 100, 200000)

	tb.bot.handlePriceMin(ctx, 100, "not-a-number")

	last := tb.msg.last()
	if last.ChatID != 100 {
		t.Errorf("expected error message to chat 100, got %d", last.ChatID)
	}
	if last.HasKB {
		t.Error("invalid input should NOT produce a keyboard")
	}
}

func TestHandlePriceMin_ExceedsMax(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "alice")

	tb.simulateCommand(ctx, 100, "/watch")

	wd := tb.bot.loadWizardData(ctx, 100)
	wd.PriceMax = 100000
	tb.bot.saveWizardState(ctx, 100, StateAskPriceMin, wd)
	tb.msg.reset()

	tb.bot.handlePriceMin(ctx, 100, "200000")

	last := tb.msg.last()
	if last.ChatID != 100 {
		t.Errorf("expected error message, got %d", last.ChatID)
	}
	if last.HasKB {
		t.Error("exceeds-max error should NOT produce a keyboard")
	}
}

func TestRegisterHandlers_NilBot(t *testing.T) {
	tb := newTestBot(t)
	tb.bot.bot = nil
	tb.bot.RegisterHandlers()
}

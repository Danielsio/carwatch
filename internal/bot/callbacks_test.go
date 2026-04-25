package bot

import (
	"context"
	"testing"
)

func TestIsValidToken(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"abc12", true},
		{"ABCDE", true},
		{"token1234567890", true},
		{"12345678901234567890", true},
		{"abcd", false},
		{"abc", false},
		{"", false},
		{"123456789012345678901", false},
		{"abc-def", false},
		{"abc def", false},
		{"abc_def", false},
		{"abc.def", false},
		{"abc!@#", false},
	}

	for _, tt := range tests {
		if got := isValidToken(tt.token); got != tt.want {
			t.Errorf("isValidToken(%q) = %v, want %v", tt.token, got, tt.want)
		}
	}
}

func newTestBotWithStores(t *testing.T) *testBot {
	t.Helper()
	tb := newTestBot(t)
	tb.bot.saved = tb.store
	tb.bot.hidden = tb.store
	return tb
}

func TestOnSaveListing_InvalidToken(t *testing.T) {
	tb := newTestBotWithStores(t)
	ctx := context.Background()
	const chatID int64 = 700
	tb.createUser(ctx, t, chatID, "alice")

	tb.bot.onSaveListing(ctx, chatID, cbPrefixSave+"ab")
	last := tb.msg.last()
	if last.Text != "Something went wrong. Please try again." {
		t.Errorf("expected error message, got %q", last.Text)
	}
}

func TestOnSaveListing_Success(t *testing.T) {
	tb := newTestBotWithStores(t)
	ctx := context.Background()
	const chatID int64 = 701
	tb.createUser(ctx, t, chatID, "bob")

	tb.bot.onSaveListing(ctx, chatID, cbPrefixSave+"abcde12345")
	last := tb.msg.last()
	if last.Text != "Saved!" {
		t.Errorf("expected 'Saved!', got %q", last.Text)
	}
}

func TestOnSaveListing_LimitReached(t *testing.T) {
	tb := newTestBotWithStores(t)
	ctx := context.Background()
	const chatID int64 = 702
	tb.createUser(ctx, t, chatID, "carol")

	for i := range maxSavedListings {
		token := "token" + padInt(i)
		_ = tb.store.SaveBookmark(ctx, chatID, token)
	}

	tb.bot.onSaveListing(ctx, chatID, cbPrefixSave+"newtoken12345")
	last := tb.msg.last()
	if last.Text != "You've reached the limit of 500 saved listings." {
		t.Errorf("expected limit message, got %q", last.Text)
	}
}

func TestOnHideListing_InvalidToken(t *testing.T) {
	tb := newTestBotWithStores(t)
	ctx := context.Background()
	const chatID int64 = 703
	tb.createUser(ctx, t, chatID, "dave")

	tb.bot.onHideListing(ctx, chatID, cbPrefixHide+"!!")
	last := tb.msg.last()
	if last.Text != "Something went wrong. Please try again." {
		t.Errorf("expected error message, got %q", last.Text)
	}
}

func TestOnHideListing_Success(t *testing.T) {
	tb := newTestBotWithStores(t)
	ctx := context.Background()
	const chatID int64 = 704
	tb.createUser(ctx, t, chatID, "eve")

	tb.bot.onHideListing(ctx, chatID, cbPrefixHide+"abcde12345")
	last := tb.msg.last()
	if last.Text != "Hidden" {
		t.Errorf("expected 'Hidden', got %q", last.Text)
	}
}

func padInt(i int) string {
	s := ""
	for n := i; n > 0 || len(s) < 5; n /= 26 {
		s += string(rune('a' + n%26))
		if len(s) >= 5 {
			break
		}
	}
	for len(s) < 5 {
		s += "a"
	}
	return s
}

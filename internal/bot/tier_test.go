package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestSearchLimitAllUsersGet10(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Create 1 search
	_, err := tb.store.CreateSearch(ctx, newTestSearch(100))
	if err != nil {
		t.Fatal(err)
	}

	// User should still be able to create more (no upgrade prompt)
	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/watch")
	last := tb.msg.last()
	if strings.Contains(last.Text, "Upgrade") || strings.Contains(last.Text, "upgrade") {
		t.Fatalf("should not see upgrade prompt, got: %s", last.Text)
	}
	if !last.HasKB {
		t.Fatal("expected source keyboard — user should be allowed to create more searches")
	}
}

func TestUpgradeCommandShowsAllFeaturesAvailable(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	tb.simulateCommand(ctx, 100, "/upgrade")
	last := tb.msg.last()
	if !strings.Contains(last.Text, "available") && !strings.Contains(last.Text, "זמינות") {
		t.Fatalf("expected all-features-available message, got: %s", last.Text)
	}
}

func TestSettingsDoesNotShowTier(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	tb.simulateCommand(ctx, 100, "/settings")
	last := tb.msg.last()
	if strings.Contains(last.Text, "Free") || strings.Contains(last.Text, "Premium") {
		t.Fatalf("settings should not show tier info, got: %s", last.Text)
	}
}

func TestDailyDigestAvailableToAllUsers(t *testing.T) {
	tb := newTestBotWithDigests(t)
	tb.bot.dailyDigests = tb.store
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Free user should see daily digest controls
	tb.simulateCommand(ctx, 100, "/digest")
	last := tb.msg.last()
	if last.Buttons < 3 {
		t.Fatalf("expected daily digest button for all users, only got %d buttons", last.Buttons)
	}
}

func TestDailyDigestCallbackWorksForAllUsers(t *testing.T) {
	tb := newTestBotWithDigests(t)
	tb.bot.dailyDigests = tb.store
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Free user enabling daily digest should succeed (no upgrade prompt)
	tb.simulateCallback(ctx, 100, cbDailyDigestOn)
	last := tb.msg.last()
	if strings.Contains(last.Text, "Premium") || strings.Contains(last.Text, "upgrade") {
		t.Fatalf("should not see upgrade prompt, got: %s", last.Text)
	}
}

func newTestSearch(chatID int64) storage.Search {
	return storage.Search{
		ChatID:       chatID,
		Name:         "test-search",
		Source:       "yad2",
		Manufacturer: 19,
		Model:        8640,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     200000,
	}
}

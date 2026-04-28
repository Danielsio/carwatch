package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestSearchLimit_AllowsUpTo10(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	for i := range 9 {
		_, err := tb.store.CreateSearch(ctx, storage.Search{
			ChatID: 100, Name: fmt.Sprintf("search-%d", i),
			Source: "yad2", Manufacturer: 19, Model: 8640,
			YearMin: 2018, YearMax: 2024, PriceMax: 200000,
		})
		if err != nil {
			t.Fatalf("create search %d: %v", i, err)
		}
	}

	// 10th search should still be allowed
	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/watch")
	last := tb.msg.last()
	if strings.Contains(last.Text, "Upgrade") || strings.Contains(last.Text, "upgrade") {
		t.Fatalf("should not see upgrade prompt with 9 searches, got: %s", last.Text)
	}
	if !last.HasKB {
		t.Fatal("expected source keyboard — 10th search should be allowed")
	}
}

func TestSearchLimit_BlocksAt10(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	for i := range 10 {
		_, err := tb.store.CreateSearch(ctx, storage.Search{
			ChatID: 100, Name: fmt.Sprintf("search-%d", i),
			Source: "yad2", Manufacturer: 19, Model: 8640,
			YearMin: 2018, YearMax: 2024, PriceMax: 200000,
		})
		if err != nil {
			t.Fatalf("create search %d: %v", i, err)
		}
	}

	// 11th search should be blocked
	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/watch")
	last := tb.msg.last()
	if last.HasKB {
		t.Fatal("expected no keyboard — 11th search should be blocked")
	}
	if !strings.Contains(last.Text, "10") || !strings.Contains(last.Text, "max") {
		t.Fatalf("expected limit-reached message mentioning max and 10, got: %s", last.Text)
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

func TestDailyDigestVisibleToFreeUsers(t *testing.T) {
	tb := newTestBotWithDigests(t)
	tb.bot.dailyDigests = tb.store
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	tb.simulateCommand(ctx, 100, "/digest")
	last := tb.msg.last()
	if last.Buttons < 3 {
		t.Fatalf("expected daily digest button for all users, only got %d buttons", last.Buttons)
	}
	text := strings.ToLower(last.Text)
	if !strings.Contains(text, "digest") && !strings.Contains(text, "daily") &&
		!strings.Contains(text, "סיכום") {
		t.Fatalf("expected digest-related content in response, got: %s", last.Text)
	}
}

func TestDailyDigestCallbackEnablesDigest(t *testing.T) {
	tb := newTestBotWithDigests(t)
	tb.bot.dailyDigests = tb.store
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	tb.simulateCallback(ctx, 100, cbDailyDigestOn)
	last := tb.msg.last()
	if strings.Contains(last.Text, "Premium") || strings.Contains(last.Text, "upgrade") {
		t.Fatalf("should not see upgrade prompt, got: %s", last.Text)
	}

	enabled, _, _, err := tb.store.GetDailyDigest(ctx, 100)
	if err != nil {
		t.Fatalf("GetDailyDigest: %v", err)
	}
	if !enabled {
		t.Fatal("daily digest should be enabled after callback")
	}
}

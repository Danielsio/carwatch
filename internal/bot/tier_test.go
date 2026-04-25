package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestFreeUserSearchLimit(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Free user should be able to create 1 search
	tb.simulateCommand(ctx, 100, "/watch")
	if !tb.msg.last().HasKB {
		t.Fatal("expected source keyboard for first search")
	}

	// Create a search manually
	_, err := tb.store.CreateSearch(ctx, newTestSearch(100))
	if err != nil {
		t.Fatal(err)
	}

	// Second search should be blocked for free user
	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/watch")
	last := tb.msg.last()
	if !strings.Contains(last.Text, "Upgrade") && !strings.Contains(last.Text, "upgrade") {
		t.Fatalf("expected upgrade prompt, got: %s", last.Text)
	}
}

func TestPremiumUserSearchLimit(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Grant premium
	expires := time.Now().Add(30 * 24 * time.Hour)
	if err := tb.store.SetUserTier(ctx, 100, TierPremium, expires); err != nil {
		t.Fatal(err)
	}

	// Create 1 search
	_, err := tb.store.CreateSearch(ctx, newTestSearch(100))
	if err != nil {
		t.Fatal(err)
	}

	// Premium user should be able to create more
	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/watch")
	last := tb.msg.last()
	if strings.Contains(last.Text, "Upgrade") || strings.Contains(last.Text, "upgrade") {
		t.Fatal("premium user should not see upgrade prompt")
	}
	if !last.HasKB {
		t.Fatal("expected source keyboard for premium user")
	}
}

func TestUpgradeCommand(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	tb.simulateCommand(ctx, 100, "/upgrade")
	last := tb.msg.last()
	if !strings.Contains(last.Text, "Premium") {
		t.Fatalf("expected premium info, got: %s", last.Text)
	}
	if !strings.Contains(last.Text, "29") {
		t.Fatalf("expected price in upgrade info, got: %s", last.Text)
	}
}

func TestSettingsShowsTier(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Free user
	tb.simulateCommand(ctx, 100, "/settings")
	last := tb.msg.last()
	if !strings.Contains(last.Text, "Free") {
		t.Fatalf("expected Free tier in settings, got: %s", last.Text)
	}

	// Grant premium
	expires := time.Now().Add(30 * 24 * time.Hour)
	if err := tb.store.SetUserTier(ctx, 100, TierPremium, expires); err != nil {
		t.Fatal(err)
	}

	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/settings")
	last = tb.msg.last()
	if !strings.Contains(last.Text, "Premium") {
		t.Fatalf("expected Premium tier in settings, got: %s", last.Text)
	}
}

func TestGrantPremiumAdmin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 999, "admin")
	tb.createUser(ctx, t, 100, "user1")

	// Admin grants premium for 30 days
	tb.simulateCommand(ctx, 999, "/grant_premium 100 30")
	last := tb.msg.last()
	if !strings.Contains(last.Text, "activated") {
		t.Fatalf("expected grant success, got: %s", last.Text)
	}

	// Verify user is now premium
	user, err := tb.store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier != TierPremium {
		t.Fatalf("expected premium tier, got: %s", user.Tier)
	}
}

func TestGrantPremiumNonAdmin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	tb.simulateCommand(ctx, 100, "/grant_premium 100 30")
	last := tb.msg.last()
	if !strings.Contains(last.Text, "understand") {
		t.Fatalf("expected unknown command for non-admin, got: %s", last.Text)
	}
}

func TestRevokePremiumAdmin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	tb.createUser(ctx, t, 999, "admin")
	tb.createUser(ctx, t, 100, "user1")

	// Grant then revoke
	expires := time.Now().Add(30 * 24 * time.Hour)
	if err := tb.store.SetUserTier(ctx, 100, TierPremium, expires); err != nil {
		t.Fatal(err)
	}

	tb.simulateCommand(ctx, 999, "/revoke_premium 100")
	last := tb.msg.last()
	if !strings.Contains(last.Text, "revoked") {
		t.Fatalf("expected revoke success, got: %s", last.Text)
	}

	user, err := tb.store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier != TierFree {
		t.Fatalf("expected free tier after revoke, got: %s", user.Tier)
	}
}

func TestTrialGrantOnStart(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	tb.simulateCommand(ctx, 100, "/start")

	// Check trial message was sent (Hebrew: "ניסיון" or "פרימיום")
	found := false
	for _, m := range tb.msg.messages {
		if strings.Contains(m.Text, "trial") || strings.Contains(m.Text, "Trial") ||
			strings.Contains(m.Text, "Premium") || strings.Contains(m.Text, "פרימיום") ||
			strings.Contains(m.Text, "🎉") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected trial welcome message on first /start")
	}

	// User should be premium
	user, err := tb.store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier != TierPremium {
		t.Fatalf("expected premium tier after trial grant, got: %s", user.Tier)
	}
	if !user.TrialUsed {
		t.Fatal("expected trial_used to be true")
	}
}

func TestTrialNotGrantedOnSecondStart(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	// First /start grants trial
	tb.simulateCommand(ctx, 100, "/start")

	user, err := tb.store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier != TierPremium {
		t.Fatalf("expected premium after first /start, got: %s", user.Tier)
	}
	firstExpiry := user.TierExpires

	// Downgrade to free (simulating expiry)
	if err := tb.store.SetUserTier(ctx, 100, TierFree, time.Time{}); err != nil {
		t.Fatal(err)
	}

	// Second /start should NOT re-grant trial (trial_used is already true)
	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/start")

	user, err = tb.store.GetUser(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if user.Tier == TierPremium {
		t.Fatal("should not re-grant trial on second /start")
	}
	_ = firstExpiry
}

func TestDailyDigestGatedBehindPremium(t *testing.T) {
	tb := newTestBotWithDigests(t)
	tb.bot.dailyDigests = tb.store
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Free user should not see daily digest button
	tb.simulateCommand(ctx, 100, "/digest")
	last := tb.msg.last()
	if strings.Contains(last.Text, "Daily") || strings.Contains(last.Text, "daily") {
		t.Fatalf("free user should not see daily digest option, got: %s", last.Text)
	}

	// Grant premium
	expires := time.Now().Add(30 * 24 * time.Hour)
	if err := tb.store.SetUserTier(ctx, 100, TierPremium, expires); err != nil {
		t.Fatal(err)
	}

	// Premium user should see daily digest button
	tb.msg.reset()
	tb.simulateCommand(ctx, 100, "/digest")
	last = tb.msg.last()
	if last.Buttons < 3 {
		t.Fatalf("expected daily digest button for premium user, only got %d buttons", last.Buttons)
	}
}

func TestDailyDigestCallbackGatedBehindPremium(t *testing.T) {
	tb := newTestBotWithDigests(t)
	tb.bot.dailyDigests = tb.store
	ctx := context.Background()
	tb.createUser(ctx, t, 100, "user1")

	// Free user trying to enable daily digest should get upgrade prompt
	tb.simulateCallback(ctx, 100, cbDailyDigestOn)
	last := tb.msg.last()
	if !strings.Contains(last.Text, "Premium") && !strings.Contains(last.Text, "upgrade") {
		t.Fatalf("expected upgrade prompt for free user, got: %s", last.Text)
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

package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestHistory_Empty(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 700

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/history")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "No matched listings") {
		t.Errorf("expected empty history message, got: %s", msg.Text)
	}
}

func TestHistory_ShowsListings(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 701

	tb.createUser(ctx, t, chatID, "alice")
	_, _ = tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "mazda-3", Manufacturer: 27, Model: 1,
	})

	// Save listing details to listing_history (per-user via chat_id).
	_ = tb.store.SaveListing(ctx, storage.ListingRecord{
		Token: "token-1", ChatID: chatID, SearchName: "mazda-3",
		Manufacturer: "Mazda", Model: "3", Year: 2020,
		Price: 120000, Km: 50000, Hand: 2,
		City: "Tel Aviv", PageLink: "https://example.com/1",
		FirstSeenAt: time.Now(),
	})
	_ = tb.store.SaveListing(ctx, storage.ListingRecord{
		Token: "token-2", ChatID: chatID, SearchName: "mazda-3",
		Manufacturer: "Mazda", Model: "3", Year: 2021,
		Price: 140000, Km: 30000, Hand: 1,
		City: "Haifa", PageLink: "https://example.com/2",
		FirstSeenAt: time.Now(),
	})

	tb.simulateCommand(ctx, chatID, "/history")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Match history (2 total)") {
		t.Errorf("expected history header with count 2, got: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "Mazda 3") {
		t.Errorf("expected car name in history, got: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "120,000") {
		t.Errorf("expected formatted price in history, got: %s", msg.Text)
	}
}

func TestHistory_Pagination(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 702

	tb.createUser(ctx, t, chatID, "bob")

	// Create more listings than one page (historyPageSize = 5).
	for i := range 7 {
		token := "tok-" + string(rune('a'+i))
		_ = tb.store.SaveListing(ctx, storage.ListingRecord{
			Token: token, ChatID: chatID, SearchName: "test",
			Manufacturer: "Mazda", Model: "3", Year: 2020 + i,
			Price: 100000 + i*10000, FirstSeenAt: time.Now(),
		})
	}

	tb.simulateCommand(ctx, chatID, "/history")

	msg := tb.msg.last()
	if !msg.HasKB {
		t.Fatal("expected keyboard with pagination buttons")
	}
	if !strings.Contains(msg.Text, "7 total") {
		t.Errorf("expected 7 total, got: %s", msg.Text)
	}
	if msg.Buttons < 2 {
		t.Errorf("expected at least 2 pagination buttons (page indicator + next), got %d", msg.Buttons)
	}

	// Navigate to page 2 via callback.
	tb.msg.reset()
	tb.simulateCallback(ctx, chatID, cbHistoryPage+"1")

	msg = tb.msg.last()
	if !strings.Contains(msg.Text, "Match history") {
		t.Errorf("page 2 should still show history header, got: %s", msg.Text)
	}
}

func TestHistory_IsolatedPerUser(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const alice int64 = 710
	const bob int64 = 711

	tb.createUser(ctx, t, alice, "alice")
	tb.createUser(ctx, t, bob, "bob")

	_ = tb.store.SaveListing(ctx, storage.ListingRecord{
		Token: "alice-tok", ChatID: alice, SearchName: "a-search",
		Manufacturer: "Mazda", Model: "3", Year: 2020,
		Price: 100000, FirstSeenAt: time.Now(),
	})

	_ = tb.store.SaveListing(ctx, storage.ListingRecord{
		Token: "bob-tok", ChatID: bob, SearchName: "b-search",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2019,
		Price: 90000, FirstSeenAt: time.Now(),
	})

	// Alice should see only her listing.
	tb.simulateCommand(ctx, alice, "/history")
	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Mazda") {
		t.Errorf("alice should see Mazda, got: %s", msg.Text)
	}
	if strings.Contains(msg.Text, "Toyota") {
		t.Errorf("alice should not see bob's Toyota listing")
	}

	// Bob should see only his listing.
	tb.msg.reset()
	tb.simulateCommand(ctx, bob, "/history")
	msg = tb.msg.last()
	if !strings.Contains(msg.Text, "Toyota") {
		t.Errorf("bob should see Toyota, got: %s", msg.Text)
	}
	if strings.Contains(msg.Text, "Mazda") {
		t.Errorf("bob should not see alice's Mazda listing")
	}
}

func TestHistory_InvalidPage(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 720

	tb.createUser(ctx, t, chatID, "dave")
	_ = tb.store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-1", ChatID: chatID, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2020,
		Price: 100000, FirstSeenAt: time.Now(),
	})

	// Out-of-range page should show friendly error.
	tb.simulateCallback(ctx, chatID, cbHistoryPage+"999")
	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "no longer available") {
		t.Errorf("expected out-of-range message, got: %s", msg.Text)
	}

	// Negative page should show friendly error.
	tb.msg.reset()
	tb.simulateCallback(ctx, chatID, cbHistoryPage+"-1")
	msg = tb.msg.last()
	if !strings.Contains(msg.Text, "no longer available") {
		t.Errorf("expected out-of-range message for negative page, got: %s", msg.Text)
	}
}

func TestHistory_SurvivesPrune(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 740

	tb.createUser(ctx, t, chatID, "frank")
	_, _ = tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "test", Manufacturer: 27, Model: 1,
	})

	// Claim a listing (populates seen_listings) and save history.
	_, _ = tb.store.ClaimNew(ctx, "tok-prune", chatID, 1)
	_ = tb.store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-prune", ChatID: chatID, SearchName: "test",
		Manufacturer: "Mazda", Model: "3", Year: 2020,
		Price: 100000, FirstSeenAt: time.Now(),
	})

	// Prune all seen_listings (simulates 30-day expiry).
	_, _ = tb.store.Prune(ctx, 0)

	// History should still work because it no longer depends on seen_listings.
	tb.simulateCommand(ctx, chatID, "/history")
	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Match history (1 total)") {
		t.Errorf("history should survive prune, got: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "Mazda 3") {
		t.Errorf("expected car name after prune, got: %s", msg.Text)
	}
}

func TestHistory_MarkdownEscaping(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 730

	tb.createUser(ctx, t, chatID, "eve")
	_ = tb.store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-md", ChatID: chatID, SearchName: "test",
		Manufacturer: "Land_Rover", Model: "Range*Rover", Year: 2020,
		Price: 200000, City: "Tel_Aviv",
		FirstSeenAt: time.Now(),
	})

	tb.simulateCommand(ctx, chatID, "/history")
	msg := tb.msg.last()

	if strings.Contains(msg.Text, "Land_Rover") {
		t.Error("underscores in manufacturer should be escaped")
	}
	if !strings.Contains(msg.Text, "Land\\_Rover") {
		t.Errorf("expected escaped manufacturer, got: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "Tel\\_Aviv") {
		t.Errorf("expected escaped city, got: %s", msg.Text)
	}
}

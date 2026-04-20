package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestHandleShare_NoArg(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/share")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Usage") {
		t.Errorf("expected usage message, got %q", msg.Text)
	}
}

func TestHandleShare_InvalidID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/share abc")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Invalid") {
		t.Errorf("expected invalid ID message, got %q", msg.Text)
	}
}

func TestHandleShare_NonexistentSearch(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/share 999")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", msg.Text)
	}
}

func TestHandleShare_OtherUsersSearch(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	_ = tb.store.UpsertUser(ctx, 100, "alice")
	_ = tb.store.UpsertUser(ctx, 200, "bob")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: 200, Name: "bob-search", Manufacturer: 27, Model: 10332,
		YearMin: 2020, YearMax: 2024, PriceMax: 100000,
	})
	tb.msg.reset()

	tb.simulateCommand(ctx, 100, fmt.Sprintf("/share %d", id))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("should not allow sharing another user's search, got %q", msg.Text)
	}
}

func TestHandleShare_Success(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "mazda-3", Manufacturer: 27, Model: 10332,
		YearMin: 2020, YearMax: 2024, PriceMax: 100000,
	})
	tb.msg.reset()

	tb.simulateCommand(ctx, chatID, fmt.Sprintf("/share %d", id))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "t.me/test_bot") {
		t.Errorf("expected share link with bot username, got %q", msg.Text)
	}
	if !strings.Contains(msg.Text, "Mazda") {
		t.Errorf("expected manufacturer name in share message, got %q", msg.Text)
	}
}

func TestHandleShare_NoBotUsername(t *testing.T) {
	tb := newTestBot(t)
	tb.bot.botUsername = ""
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/share 1")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not configured") {
		t.Errorf("expected 'not configured' message, got %q", msg.Text)
	}
}

func TestHandleShareStart_ValidLink(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	_ = tb.store.UpsertUser(ctx, 100, "alice")
	_ = tb.store.UpsertUser(ctx, 200, "bob")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "mazda-3", Manufacturer: 27, Model: 10332,
		YearMin: 2020, YearMax: 2024, PriceMax: 100000, EngineMinCC: 2000,
	})
	tb.msg.reset()

	tb.simulateCommand(ctx, 200, fmt.Sprintf("/start share_%d", id))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Shared search") {
		t.Errorf("expected shared search summary, got %q", msg.Text)
	}
	if !msg.HasKB {
		t.Error("expected copy button keyboard")
	}
}

func TestHandleShareStart_DeletedSearch(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	_ = tb.store.UpsertUser(ctx, 100, "alice")
	_ = tb.store.UpsertUser(ctx, 200, "bob")

	id, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "temp", Manufacturer: 27, Model: 10332,
	})
	_ = tb.store.DeleteSearch(ctx, id, 100)
	tb.msg.reset()

	tb.simulateCommand(ctx, 200, fmt.Sprintf("/start share_%d", id))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("expected 'not found' message for deleted search, got %q", msg.Text)
	}
}

func TestHandleShareStart_InvalidParam(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/start share_abc")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Invalid") {
		t.Errorf("expected invalid share link message, got %q", msg.Text)
	}
}

func TestOnShareCopy_Success(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	_ = tb.store.UpsertUser(ctx, 100, "alice")
	_ = tb.store.UpsertUser(ctx, 200, "bob")

	srcID, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "mazda-3", Source: "yad2", Manufacturer: 27, Model: 10332,
		YearMin: 2020, YearMax: 2024, PriceMax: 100000, EngineMinCC: 2000,
	})
	tb.msg.reset()

	tb.simulateCallback(ctx, 200, cbPrefixShareCopy+fmt.Sprintf("%d", srcID))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "saved") {
		t.Errorf("expected 'saved' message, got %q", msg.Text)
	}

	bobSearches, _ := tb.store.ListSearches(ctx, 200)
	if len(bobSearches) != 1 {
		t.Fatalf("bob should have 1 search, got %d", len(bobSearches))
	}
	if bobSearches[0].Manufacturer != 27 {
		t.Errorf("cloned search manufacturer = %d, want 27", bobSearches[0].Manufacturer)
	}
}

func TestOnShareCopy_AtMaxSearches(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	_ = tb.store.UpsertUser(ctx, 100, "alice")
	_ = tb.store.UpsertUser(ctx, 200, "bob")

	srcID, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "shared", Manufacturer: 27, Model: 10332,
	})

	for i := range 3 {
		_, _ = tb.store.CreateSearch(ctx, storage.Search{
			ChatID: 200, Name: fmt.Sprintf("bob-%d", i), Manufacturer: i + 1, Model: 1,
		})
	}
	tb.msg.reset()

	tb.simulateCallback(ctx, 200, cbPrefixShareCopy+fmt.Sprintf("%d", srcID))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "max") {
		t.Errorf("expected rate limit message, got %q", msg.Text)
	}
}

func TestOnShareCopy_DeletedSource(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	_ = tb.store.UpsertUser(ctx, 100, "alice")
	_ = tb.store.UpsertUser(ctx, 200, "bob")

	srcID, _ := tb.store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "temp", Manufacturer: 27, Model: 10332,
	})
	_ = tb.store.DeleteSearch(ctx, srcID, 100)
	tb.msg.reset()

	tb.simulateCallback(ctx, 200, cbPrefixShareCopy+fmt.Sprintf("%d", srcID))

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", msg.Text)
	}
}

func TestOnShareCopy_InvalidID(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	_ = tb.store.UpsertUser(ctx, chatID, "alice")
	tb.simulateCallback(ctx, chatID, cbPrefixShareCopy+"notanumber")

	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "Invalid") {
		t.Errorf("expected invalid share link message, got %q", msg.Text)
	}
}

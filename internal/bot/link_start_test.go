package bot

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
)

type stubLinkTokens struct {
	consumed []string
}

func (s *stubLinkTokens) CreateLinkToken(context.Context, int64) (string, error) {
	return "", nil
}

func (s *stubLinkTokens) ConsumeLinkToken(_ context.Context, token string) (int64, error) {
	s.consumed = append(s.consumed, token)
	return 42, nil
}

func TestHandleLinkStart_RejectsNonPrivateChat(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	mm := &mockMessenger{}
	lt := &stubLinkTokens{}
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))

	b := &Bot{
		msg:         mm,
		users:       store,
		searches:    store,
		listings:    store,
		catalog:     catalog.NewStatic(),
		adminChatID: 999,
		maxSearches: defaultMaxSearches,
		botUsername: "test_bot",
		logger:      logger,
		linkTokens:  lt,
	}

	validToken := "link_" + strings.Repeat("a", 32)
	b.handleLinkStart(ctx, -1001234567890, validToken)

	if len(lt.consumed) != 0 {
		t.Fatalf("ConsumeLinkToken called %d times, want 0 for group chat", len(lt.consumed))
	}
	last := mm.last()
	if last.Text == "" || !strings.Contains(last.Text, "❌") {
		t.Fatalf("expected error reply, got %q", last.Text)
	}
}

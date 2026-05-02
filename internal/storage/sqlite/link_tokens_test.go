package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestLinkTokens_CreateAndConsume(t *testing.T) {
	ctx := context.Background()
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	const webID int64 = 2_000_000_000_001
	token, err := s.CreateLinkToken(ctx, webID)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 32 {
		t.Fatalf("expected 32-char hex token, got len %d", len(token))
	}

	got, err := s.ConsumeLinkToken(ctx, token)
	if err != nil || got != webID {
		t.Fatalf("consume: id=%d err=%v", got, err)
	}

	_, err = s.ConsumeLinkToken(ctx, token)
	if !errors.Is(err, storage.ErrLinkTokenUsed) {
		t.Fatalf("expected used error, got %v", err)
	}
}

func TestLinkTokens_Expired(t *testing.T) {
	ctx := context.Background()
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	token, err := generateLinkTokenBytes()
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(-time.Minute)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO link_tokens (token, web_chat_id, expires_at, used)
		VALUES (?, ?, ?, 0)`,
		token, int64(999), expires)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.ConsumeLinkToken(ctx, token)
	if !errors.Is(err, storage.ErrLinkTokenExpired) {
		t.Fatalf("expected expired, got %v", err)
	}
}

func TestLinkTokens_NotFound(t *testing.T) {
	ctx := context.Background()
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, err = s.ConsumeLinkToken(ctx, "deadbeefdeadbeefdeadbeefdeadbeef")
	if !errors.Is(err, storage.ErrLinkTokenNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestLinkTelegramToWeb(t *testing.T) {
	ctx := context.Background()
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	const webID int64 = 2_000_000_000_077
	const tgID int64 = 555
	if err := s.UpsertUser(ctx, tgID, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkTelegramToWeb(ctx, tgID, webID); err != nil {
		t.Fatal(err)
	}
	u, err := s.GetLinkedTelegramUser(ctx, webID)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.ChatID != tgID || u.Username != "alice" {
		t.Fatalf("linked user: %+v err=%v", u, err)
	}

	// Reassigning same web to another telegram clears previous link on the other row
	const tg2 int64 = 556
	if err := s.UpsertUser(ctx, tg2, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkTelegramToWeb(ctx, tg2, webID); err != nil {
		t.Fatal(err)
	}
	u, err = s.GetLinkedTelegramUser(ctx, webID)
	if err != nil || u == nil || u.ChatID != tg2 {
		t.Fatalf("expected tg2 linked, got %+v", u)
	}
}

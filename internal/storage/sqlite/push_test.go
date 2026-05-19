package sqlite

import (
	"context"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestPushSubscription_SaveAndList(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	sub := storage.PushSubscription{
		ChatID:   100,
		Endpoint: "https://push.example.com/sub1",
		P256DH:   "BNcRd...",
		Auth:     "tBHI...",
	}
	if err := store.SavePushSubscription(ctx, sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	subs, err := store.ListPushSubscriptions(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Endpoint != sub.Endpoint {
		t.Errorf("endpoint = %q, want %q", subs[0].Endpoint, sub.Endpoint)
	}
	if subs[0].P256DH != sub.P256DH {
		t.Errorf("p256dh = %q, want %q", subs[0].P256DH, sub.P256DH)
	}
	if subs[0].Auth != sub.Auth {
		t.Errorf("auth = %q, want %q", subs[0].Auth, sub.Auth)
	}
	if subs[0].ChatID != 100 {
		t.Errorf("chat_id = %d, want 100", subs[0].ChatID)
	}
	if subs[0].ID == 0 {
		t.Error("expected non-zero ID")
	}
	if subs[0].CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

func TestPushSubscription_Upsert(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	sub := storage.PushSubscription{
		ChatID:   100,
		Endpoint: "https://push.example.com/upsert",
		P256DH:   "old-key",
		Auth:     "old-auth",
	}
	if err := store.SavePushSubscription(ctx, sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Upsert same endpoint with new keys
	sub.P256DH = "new-key"
	sub.Auth = "new-auth"
	if err := store.SavePushSubscription(ctx, sub); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	subs, err := store.ListPushSubscriptions(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 after upsert, got %d", len(subs))
	}
	if subs[0].P256DH != "new-key" {
		t.Errorf("p256dh = %q after upsert, want new-key", subs[0].P256DH)
	}
	if subs[0].Auth != "new-auth" {
		t.Errorf("auth = %q after upsert, want new-auth", subs[0].Auth)
	}
}

func TestPushSubscription_Delete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	sub := storage.PushSubscription{
		ChatID:   100,
		Endpoint: "https://push.example.com/del",
		P256DH:   "key",
		Auth:     "auth",
	}
	if err := store.SavePushSubscription(ctx, sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.DeletePushSubscription(ctx, 100, sub.Endpoint); err != nil {
		t.Fatalf("delete: %v", err)
	}

	subs, err := store.ListPushSubscriptions(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(subs))
	}
}

func TestPushSubscription_DeleteWrongUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	sub := storage.PushSubscription{
		ChatID:   100,
		Endpoint: "https://push.example.com/wrong",
		P256DH:   "key",
		Auth:     "auth",
	}
	if err := store.SavePushSubscription(ctx, sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	// User 200 tries to delete user 100's subscription — should not remove it
	if err := store.DeletePushSubscription(ctx, 200, sub.Endpoint); err != nil {
		t.Fatalf("delete: %v", err)
	}

	subs, err := store.ListPushSubscriptions(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 (not deleted by wrong user), got %d", len(subs))
	}
}

func TestPushSubscription_ListEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)

	subs, err := store.ListPushSubscriptions(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(subs))
	}
}

func TestPushSubscription_MultipleUsers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedUser(t, store, 100)
	seedUser(t, store, 200)

	if err := store.SavePushSubscription(ctx, storage.PushSubscription{
		ChatID: 100, Endpoint: "https://push.example.com/user100", P256DH: "k1", Auth: "a1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePushSubscription(ctx, storage.PushSubscription{
		ChatID: 200, Endpoint: "https://push.example.com/user200", P256DH: "k2", Auth: "a2",
	}); err != nil {
		t.Fatal(err)
	}

	subs100, err := store.ListPushSubscriptions(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs100) != 1 {
		t.Errorf("user 100: expected 1, got %d", len(subs100))
	}

	subs200, err := store.ListPushSubscriptions(ctx, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs200) != 1 {
		t.Errorf("user 200: expected 1, got %d", len(subs200))
	}
}

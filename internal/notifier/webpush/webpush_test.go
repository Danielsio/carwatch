package webpush

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	wp "github.com/SherClockHolmes/webpush-go"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
)

// --- mock subscription store ---

type mockSubStore struct {
	mu     sync.Mutex
	subs   map[int64][]PushSubscription
	delErr error
	// deleted tracks (chatID, endpoint) pairs removed via DeletePushSubscription.
	deleted []struct {
		chatID   int64
		endpoint string
	}
}

func newMockStore(subs map[int64][]PushSubscription) *mockSubStore {
	return &mockSubStore{subs: subs}
}

func (m *mockSubStore) ListPushSubscriptions(_ context.Context, chatID int64) ([]PushSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.subs[chatID], nil
}

func (m *mockSubStore) DeletePushSubscription(_ context.Context, chatID int64, endpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.delErr != nil {
		return m.delErr
	}
	m.deleted = append(m.deleted, struct {
		chatID   int64
		endpoint string
	}{chatID, endpoint})
	// Also remove from the in-memory map so subsequent calls reflect the deletion.
	remaining := make([]PushSubscription, 0, len(m.subs[chatID]))
	for _, s := range m.subs[chatID] {
		if s.Endpoint != endpoint {
			remaining = append(remaining, s)
		}
	}
	m.subs[chatID] = remaining
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// fakeSendFunc returns a closure that records calls and responds with the
// given status code. The returned atomic counter tracks the number of calls.
func fakeSendFunc(statusCode int) (func([]byte, *wp.Subscription, *wp.Options) (*http.Response, error), *atomic.Int32, *[]*sentMessage) {
	var calls atomic.Int32
	var mu sync.Mutex
	var messages []*sentMessage

	fn := func(msg []byte, s *wp.Subscription, _ *wp.Options) (*http.Response, error) {
		calls.Add(1)
		mu.Lock()
		messages = append(messages, &sentMessage{payload: msg, sub: s})
		mu.Unlock()
		return &http.Response{
			StatusCode: statusCode,
			Body:       http.NoBody,
		}, nil
	}
	return fn, &calls, &messages
}

type sentMessage struct {
	payload []byte
	sub     *wp.Subscription
}

// --- Connect / Disconnect ---

func TestConnect_ReturnsNil(t *testing.T) {
	n := New(newMockStore(nil), "pub", "priv", "test@example.com", testLogger())
	if err := n.Connect(context.Background()); err != nil {
		t.Errorf("Connect should return nil, got: %v", err)
	}
}

func TestDisconnect_ReturnsNil(t *testing.T) {
	n := New(newMockStore(nil), "pub", "priv", "test@example.com", testLogger())
	if err := n.Disconnect(); err != nil {
		t.Errorf("Disconnect should return nil, got: %v", err)
	}
}

// --- Notify with no subscriptions ---

func TestNotify_NoSubscriptions_Noop(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	sendFn, calls, _ := fakeSendFunc(http.StatusCreated) //nolint:bodyclose // closed inside deliver()
	n.sendFunc = sendFn

	err := n.Notify(context.Background(), "42", []model.Listing{
		{RawListing: model.RawListing{Token: "abc", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 120000}},
	}, locale.English)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 0 {
		t.Errorf("expected 0 send calls for user without subscriptions, got %d", calls.Load())
	}
}

// --- Notify with subscriptions ---

func TestNotify_SingleListing_FormatsPayload(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{
		42: {{Endpoint: "https://push.example.com/sub1", P256DH: "key1", Auth: "auth1"}},
	})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	sendFn, calls, msgs := fakeSendFunc(http.StatusCreated) //nolint:bodyclose // closed inside deliver()
	n.sendFunc = sendFn

	listings := []model.Listing{
		{
			RawListing: model.RawListing{
				Token: "tok123", Manufacturer: "Toyota", Model: "Corolla",
				Year: 2021, Price: 120000, Km: 30000,
			},
		},
	}

	err := n.Notify(context.Background(), "42", listings, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 send call, got %d", calls.Load())
	}

	var payload pushPayload
	if err := json.Unmarshal((*msgs)[0].payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Title != "Toyota Corolla 2021" {
		t.Errorf("title = %q, want %q", payload.Title, "Toyota Corolla 2021")
	}
	if payload.URL != "/listings/tok123" {
		t.Errorf("url = %q, want %q", payload.URL, "/listings/tok123")
	}
	if payload.Icon != "/icon-192.png" {
		t.Errorf("icon = %q, want %q", payload.Icon, "/icon-192.png")
	}
	// Body should contain price and km.
	if payload.Body != "120000 NIS | 30000 km" {
		t.Errorf("body = %q, want %q", payload.Body, "120000 NIS | 30000 km")
	}

	// Verify subscription keys were forwarded.
	sub := (*msgs)[0].sub
	if sub.Endpoint != "https://push.example.com/sub1" {
		t.Errorf("endpoint = %q", sub.Endpoint)
	}
	if sub.Keys.P256dh != "key1" || sub.Keys.Auth != "auth1" {
		t.Errorf("keys = %+v", sub.Keys)
	}
}

func TestNotify_MultipleListings_SummaryPayload(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{
		10: {{Endpoint: "https://push.example.com/s1", P256DH: "k", Auth: "a"}},
	})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	sendFn, _, msgs := fakeSendFunc(http.StatusCreated) //nolint:bodyclose // closed inside deliver()
	n.sendFunc = sendFn

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000}},
		{RawListing: model.RawListing{Token: "b", Manufacturer: "Honda", Model: "Civic", Year: 2022, Price: 110000}},
		{RawListing: model.RawListing{Token: "c", Manufacturer: "Mazda", Model: "3", Year: 2020, Price: 90000}},
	}

	err := n.Notify(context.Background(), "10", listings, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload pushPayload
	if err := json.Unmarshal((*msgs)[0].payload, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Title != "3 new listings" {
		t.Errorf("title = %q, want %q", payload.Title, "3 new listings")
	}
	if payload.Body != "Toyota Corolla and 2 more" {
		t.Errorf("body = %q, want %q", payload.Body, "Toyota Corolla and 2 more")
	}
}

// --- NotifyRaw ---

func TestNotifyRaw_FormatsPayload(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{
		7: {{Endpoint: "https://push.example.com/s1", P256DH: "k", Auth: "a"}},
	})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	sendFn, _, msgs := fakeSendFunc(http.StatusCreated) //nolint:bodyclose // closed inside deliver()
	n.sendFunc = sendFn

	err := n.NotifyRaw(context.Background(), "7", "Price dropped on Toyota Corolla!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload pushPayload
	if err := json.Unmarshal((*msgs)[0].payload, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Title != "CarWatch" {
		t.Errorf("title = %q, want %q", payload.Title, "CarWatch")
	}
	if payload.Body != "Price dropped on Toyota Corolla!" {
		t.Errorf("body = %q, want %q", payload.Body, "Price dropped on Toyota Corolla!")
	}
}

func TestNotifyRaw_NoSubscriptions_Noop(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	sendFn, calls, _ := fakeSendFunc(http.StatusCreated) //nolint:bodyclose // closed inside deliver()
	n.sendFunc = sendFn

	err := n.NotifyRaw(context.Background(), "99", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 0 {
		t.Errorf("expected 0 calls, got %d", calls.Load())
	}
}

// --- Invalid recipient ---

func TestNotify_InvalidRecipient(t *testing.T) {
	n := New(newMockStore(nil), "pub", "priv", "test@example.com", testLogger())
	err := n.Notify(context.Background(), "not-a-number", nil, locale.English)
	if err == nil {
		t.Fatal("expected error for invalid recipient")
	}
}

func TestNotifyRaw_InvalidRecipient(t *testing.T) {
	n := New(newMockStore(nil), "pub", "priv", "test@example.com", testLogger())
	err := n.NotifyRaw(context.Background(), "abc", "hello")
	if err == nil {
		t.Fatal("expected error for invalid recipient")
	}
}

// --- Gone subscription cleanup ---

func TestDeliver_GoneSubscription_Deleted(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{
		1: {
			{Endpoint: "https://push.example.com/gone", P256DH: "k", Auth: "a"},
			{Endpoint: "https://push.example.com/ok", P256DH: "k2", Auth: "a2"},
		},
	})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	callNum := atomic.Int32{}
	n.sendFunc = func(msg []byte, s *wp.Subscription, _ *wp.Options) (*http.Response, error) {
		c := callNum.Add(1)
		status := http.StatusCreated
		if c == 1 {
			status = http.StatusGone
		}
		return &http.Response{StatusCode: status, Body: http.NoBody}, nil
	}

	err := n.Notify(context.Background(), "1", []model.Listing{
		{RawListing: model.RawListing{Token: "x", Manufacturer: "Test", Model: "Car", Price: 100000}},
	}, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.deleted) != 1 {
		t.Fatalf("expected 1 deletion, got %d", len(store.deleted))
	}
	if store.deleted[0].endpoint != "https://push.example.com/gone" {
		t.Errorf("deleted wrong endpoint: %q", store.deleted[0].endpoint)
	}
}

// --- Rate limited subscription ---

func TestDeliver_RateLimited_Skipped(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{
		1: {{Endpoint: "https://push.example.com/rl", P256DH: "k", Auth: "a"}},
	})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	sendFn, _, _ := fakeSendFunc(http.StatusTooManyRequests) //nolint:bodyclose // closed inside deliver()
	n.sendFunc = sendFn

	err := n.NotifyRaw(context.Background(), "1", "rate limit test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.deleted) != 0 {
		t.Errorf("should not delete rate-limited subscription, got %d deletions", len(store.deleted))
	}
}

// --- Send error ---

func TestDeliver_SendError_ReturnsFirstError(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{
		1: {
			{Endpoint: "https://push.example.com/fail1", P256DH: "k", Auth: "a"},
			{Endpoint: "https://push.example.com/fail2", P256DH: "k2", Auth: "a2"},
		},
	})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	n.sendFunc = func(_ []byte, _ *wp.Subscription, _ *wp.Options) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	}

	err := n.NotifyRaw(context.Background(), "1", "error test message here")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "webpush: send to 1: network error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Multiple subscriptions fan-out ---

func TestDeliver_MultipleSubscriptions_FansOut(t *testing.T) {
	store := newMockStore(map[int64][]PushSubscription{
		5: {
			{Endpoint: "https://push.example.com/s1", P256DH: "k1", Auth: "a1"},
			{Endpoint: "https://push.example.com/s2", P256DH: "k2", Auth: "a2"},
			{Endpoint: "https://push.example.com/s3", P256DH: "k3", Auth: "a3"},
		},
	})
	n := New(store, "pub", "priv", "test@example.com", testLogger())

	sendFn, calls, _ := fakeSendFunc(http.StatusCreated) //nolint:bodyclose // closed inside deliver()
	n.sendFunc = sendFn

	err := n.Notify(context.Background(), "5", []model.Listing{
		{RawListing: model.RawListing{Token: "t", Manufacturer: "BMW", Model: "3", Price: 200000}},
	}, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 send calls, got %d", calls.Load())
	}
}

// --- buildListingPayload edge cases ---

func TestBuildListingPayload_Empty(t *testing.T) {
	p := buildListingPayload(nil)
	if p.Title != "CarWatch" {
		t.Errorf("title = %q, want CarWatch", p.Title)
	}
}

func TestBuildListingPayload_NoManufacturer(t *testing.T) {
	p := buildListingPayload([]model.Listing{
		{RawListing: model.RawListing{Token: "x", Price: 50000}},
	})
	if p.Title != "New listing" {
		t.Errorf("title = %q, want %q", p.Title, "New listing")
	}
}

func TestBuildListingPayload_WithSubModel(t *testing.T) {
	p := buildListingPayload([]model.Listing{
		{RawListing: model.RawListing{Token: "x", Manufacturer: "Toyota", Model: "Corolla", SubModel: "Cross", Year: 2023, Price: 150000}},
	})
	if p.Title != "Toyota Corolla Cross 2023" {
		t.Errorf("title = %q, want %q", p.Title, "Toyota Corolla Cross 2023")
	}
}

func TestBuildListingPayload_NoPriceNoKm(t *testing.T) {
	p := buildListingPayload([]model.Listing{
		{RawListing: model.RawListing{Token: "x", Manufacturer: "Honda", Model: "Jazz"}},
	})
	if p.Body != "New listing found" {
		t.Errorf("body = %q, want %q", p.Body, "New listing found")
	}
}

// --- truncateEndpoint ---

func TestTruncateEndpoint_Short(t *testing.T) {
	ep := "https://short.com"
	if truncateEndpoint(ep) != ep {
		t.Errorf("short endpoint should not be truncated")
	}
}

func TestTruncateEndpoint_Long(t *testing.T) {
	ep := "https://fcm.googleapis.com/fcm/send/very-long-subscription-identifier-that-exceeds-sixty-characters-definitely"
	got := truncateEndpoint(ep)
	if len(got) != 63 { // 60 + "..."
		t.Errorf("truncated length = %d, want 63", len(got))
	}
}

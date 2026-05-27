package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/redis/go-redis/v9"
)

func TestInstantDelivery_DeliverBatch_Success(t *testing.T) {
	n := &mockNotifier{}
	d := NewInstantDelivery(n, locale.English)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification, got %d", len(n.messages))
	}
}

func TestInstantDelivery_DeliverBatch_NotifyFails(t *testing.T) {
	n := &mockNotifier{err: errors.New("telegram down")}
	d := NewInstantDelivery(n, locale.English)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a"}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err == nil {
		t.Fatal("expected error when notifier fails")
	}
}

func TestInstantDelivery_DeliverBatch_WithPublisherIncludesTokens(t *testing.T) {
	mr := miniredis.RunT(t)
	pub, err := broker.NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	n := &mockNotifier{}
	d := NewInstantDelivery(
		n,
		locale.English,
		WithPublisher(pub),
		WithSearchContext(55, "Search 55"),
	)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "tok-a", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000}},
		{RawListing: model.RawListing{Token: "tok-b", Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 90000}},
	}
	if err := d.DeliverBatch(context.Background(), 100, listings); err != nil {
		t.Fatalf("deliver batch: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()
	msgs, err := client.XRange(context.Background(), broker.StreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 queued message, got %d", len(msgs))
	}
	data, ok := msgs[0].Values["data"].(string)
	if !ok {
		t.Fatal("missing data field in queued message")
	}
	var queued broker.Alert
	if err := json.Unmarshal([]byte(data), &queued); err != nil {
		t.Fatalf("unmarshal queued alert: %v", err)
	}
	if queued.ChatID != 100 || queued.SearchID != 55 || queued.SearchName != "Search 55" {
		t.Fatalf("unexpected queued metadata: %+v", queued)
	}
	if len(queued.Tokens) != 2 || queued.Tokens[0] != "tok-a" || queued.Tokens[1] != "tok-b" {
		t.Fatalf("unexpected queued tokens: %+v", queued.Tokens)
	}
}

type failDigestStore struct {
	mockDigestStore
	addErr error
}

func newFailDigestStore(err error) *failDigestStore {
	return &failDigestStore{
		mockDigestStore: mockDigestStore{
			modes:   make(map[int64]struct{ mode, interval string }),
			items:   make(map[int64][]digestItem),
			flushed: make(map[int64]time.Time),
		},
		addErr: err,
	}
}

func (m *failDigestStore) AddDigestItem(_ context.Context, _ int64, _ string) error {
	return m.addErr
}

func TestInstantDelivery_DeliverRaw_Success(t *testing.T) {
	n := &mockNotifier{}
	d := NewInstantDelivery(n, locale.English)

	err := d.DeliverRaw(context.Background(), 100, "price drop!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Errorf("expected 1 raw message, got %d", len(n.rawMessages))
	}
	if n.rawMessages[0].recipient != "100" {
		t.Errorf("recipient = %q, want '100'", n.rawMessages[0].recipient)
	}
}

type errRawNotifier struct {
	mockNotifier
	rawErr error
}

func (m *errRawNotifier) NotifyRaw(_ context.Context, recipient string, message string) error {
	if m.rawErr != nil {
		return m.rawErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rawMessages = append(m.rawMessages, rawNotifyCall{recipient: recipient, message: message})
	return nil
}

func TestInstantDelivery_DeliverRaw_NotifyFails(t *testing.T) {
	n := &errRawNotifier{rawErr: errors.New("telegram down")}
	d := NewInstantDelivery(n, locale.English)

	err := d.DeliverRaw(context.Background(), 100, "price drop!")
	if err == nil {
		t.Fatal("expected error when notifier fails")
	}
}

func TestDigestDelivery_DeliverBatch(t *testing.T) {
	ds := newMockDigestStore()
	d := NewDigestDelivery(ds, locale.English)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()
	if len(ds.items[100]) != 1 {
		t.Errorf("expected 1 digest item, got %d", len(ds.items[100]))
	}
}

func TestDigestDelivery_DeliverRaw(t *testing.T) {
	ds := newMockDigestStore()
	d := NewDigestDelivery(ds, locale.English)

	err := d.DeliverRaw(context.Background(), 100, "price drop!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()
	if len(ds.items[100]) != 1 {
		t.Errorf("expected 1 digest item, got %d", len(ds.items[100]))
	}
	if ds.items[100][0].payload != "price drop!" {
		t.Errorf("item = %q, want 'price drop!'", ds.items[100][0].payload)
	}
}

func TestDigestDelivery_DeliverBatch_Error(t *testing.T) {
	ds := newFailDigestStore(errors.New("write failed"))
	d := NewDigestDelivery(ds, locale.English)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a"}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstantDelivery_DeliverRaw_BlocksMalformed(t *testing.T) {
	n := &mockNotifier{}
	d := NewInstantDelivery(n, locale.English)

	tests := []struct {
		name string
		msg  string
	}{
		{"template syntax", "{{.}}"},
		{"too short", "hi"},
		{"empty", ""},
		{"sprintf error", "%!s(MISSING)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.DeliverRaw(context.Background(), 100, tt.msg)
			if !errors.Is(err, errMalformedMessage) {
				t.Errorf("expected errMalformedMessage, got: %v", err)
			}
		})
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 0 {
		t.Errorf("no messages should reach notifier, got %d", len(n.rawMessages))
	}
}

func TestInstantDelivery_DeliverRaw_AllowsValid(t *testing.T) {
	n := &mockNotifier{}
	d := NewInstantDelivery(n, locale.English)

	err := d.DeliverRaw(context.Background(), 100, "Valid notification message here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Errorf("expected 1 raw message, got %d", len(n.rawMessages))
	}
}

func TestDigestDelivery_DeliverRaw_BlocksMalformed(t *testing.T) {
	ds := newMockDigestStore()
	d := NewDigestDelivery(ds, locale.English)

	err := d.DeliverRaw(context.Background(), 100, "{{.}}")
	if !errors.Is(err, errMalformedMessage) {
		t.Errorf("expected errMalformedMessage, got: %v", err)
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()
	if len(ds.items[100]) != 0 {
		t.Errorf("no items should be stored, got %d", len(ds.items[100]))
	}
}

package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/redis/go-redis/v9"
)

func TestSortListingsByScore(t *testing.T) {
	dealScore := func(s int) *model.ScoreInfo { return &model.ScoreInfo{Score: s} }
	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "low"}, FitnessScore: 3.0},
		{RawListing: model.RawListing{Token: "high"}, FitnessScore: 9.0},
		{RawListing: model.RawListing{Token: "mid-weak-deal"}, FitnessScore: 6.0, DealScore: dealScore(40)},
		{RawListing: model.RawListing{Token: "mid-strong-deal"}, FitnessScore: 6.0, DealScore: dealScore(80)},
	}

	sortListingsByScore(listings)

	want := []string{"high", "mid-strong-deal", "mid-weak-deal", "low"}
	for i, w := range want {
		if listings[i].Token != w {
			t.Fatalf("position %d = %q, want %q (full order: %v)", i, listings[i].Token, w, tokensOf(listings))
		}
	}
}

func TestInstantDelivery_DeliverBatch_TruncatesToBest(t *testing.T) {
	mr := miniredis.RunT(t)
	pub, err := broker.NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	d := NewInstantDelivery(&mockNotifier{}, locale.English, WithPublisher(pub), WithSearchContext(1, "s"))

	// More listings than fit in one batch, in worst-first order so a naive
	// "keep the first N" would drop the best cars.
	var listings []model.Listing
	for i := 0; i < maxBatchSize+2; i++ {
		listings = append(listings, model.Listing{
			RawListing:   model.RawListing{Token: fmt.Sprintf("tok-%02d", i), Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000},
			FitnessScore: float64(i), // higher index = better car
		})
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
	var queued broker.Alert
	if err := json.Unmarshal([]byte(msgs[0].Values["data"].(string)), &queued); err != nil {
		t.Fatalf("unmarshal queued alert: %v", err)
	}

	if len(queued.Tokens) != maxBatchSize {
		t.Fatalf("expected %d tokens after truncation, got %d", maxBatchSize, len(queued.Tokens))
	}
	if queued.Tokens[0] != "tok-11" {
		t.Errorf("best listing should lead the batch, got %q", queued.Tokens[0])
	}
	for _, dropped := range []string{"tok-00", "tok-01"} {
		for _, tok := range queued.Tokens {
			if tok == dropped {
				t.Errorf("lowest-scored listing %q should have been truncated, but was delivered", dropped)
			}
		}
	}
}

func tokensOf(listings []model.Listing) []string {
	out := make([]string, len(listings))
	for i, l := range listings {
		out[i] = l.Token
	}
	return out
}

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

func (m *failDigestStore) AddDigestItem(_ context.Context, _ int64, _ string, _ []string) error {
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

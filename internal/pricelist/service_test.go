package pricelist

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

type mockPriceListStore struct {
	entries map[string]*storage.PriceListEntry
	setErr  error
}

func newMockStore() *mockPriceListStore {
	return &mockPriceListStore{entries: make(map[string]*storage.PriceListEntry)}
}

func (m *mockPriceListStore) key(subModelID, year int) string {
	return fmt.Sprintf("%d:%d", subModelID, year)
}

func (m *mockPriceListStore) GetPriceListEntry(_ context.Context, subModelID, year int) (*storage.PriceListEntry, error) {
	e, ok := m.entries[m.key(subModelID, year)]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (m *mockPriceListStore) SetPriceListEntry(_ context.Context, e storage.PriceListEntry) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.entries[m.key(e.SubModelID, e.Year)] = &e
	return nil
}

func TestLookup_CacheHit(t *testing.T) {
	store := newMockStore()
	store.entries["100:2020"] = &storage.PriceListEntry{
		SubModelID: 100, Year: 2020, BasePrice: 80000,
		FetchedAt: time.Now().Add(-1 * time.Hour),
	}

	client := &mockHTTPDoer{statusCode: 500}
	svc := NewService(store, client, slog.Default())

	bp, ok := svc.Lookup(context.Background(), 100, 2020)
	if !ok {
		t.Fatal("expected ok=true for cache hit")
	}
	if bp != 80000 {
		t.Errorf("expected 80000, got %d", bp)
	}
}

func TestLookup_CacheMiss_FetchesAndCaches(t *testing.T) {
	store := newMockStore()
	html := `<div>מחיר בסיס ₪ 95,000</div>`
	client := &mockHTTPDoer{body: []byte(html), statusCode: 200}
	svc := NewService(store, client, slog.Default())

	bp, ok := svc.Lookup(context.Background(), 200, 2021)
	if !ok {
		t.Fatal("expected ok=true after fetch")
	}
	if bp != 95000 {
		t.Errorf("expected 95000, got %d", bp)
	}

	cached := store.entries["200:2021"]
	if cached == nil {
		t.Fatal("expected entry to be cached")
	}
	if cached.BasePrice != 95000 {
		t.Errorf("cached base_price: expected 95000, got %d", cached.BasePrice)
	}
}

func TestLookup_InvalidParams(t *testing.T) {
	svc := NewService(newMockStore(), &mockHTTPDoer{}, slog.Default())

	_, ok := svc.Lookup(context.Background(), 0, 2020)
	if ok {
		t.Error("expected ok=false for subModelID=0")
	}

	_, ok = svc.Lookup(context.Background(), 100, 0)
	if ok {
		t.Error("expected ok=false for year=0")
	}
}

func TestLookup_FetchFailsFallsBackToStaleCache(t *testing.T) {
	store := newMockStore()
	store.entries["100:2020"] = &storage.PriceListEntry{
		SubModelID: 100, Year: 2020, BasePrice: 70000,
		FetchedAt: time.Now().Add(-30 * 24 * time.Hour), // stale (>7 days)
	}

	client := &mockHTTPDoer{statusCode: 400}
	svc := NewService(store, client, slog.Default())

	bp, ok := svc.Lookup(context.Background(), 100, 2020)
	if !ok {
		t.Fatal("expected ok=true with stale fallback")
	}
	if bp != 70000 {
		t.Errorf("expected stale value 70000, got %d", bp)
	}
}

func TestLookup_CachedZeroTreatedAsMiss(t *testing.T) {
	store := newMockStore()
	store.entries["100:2020"] = &storage.PriceListEntry{
		SubModelID: 100, Year: 2020, BasePrice: 0,
		FetchedAt: time.Now().Add(-1 * time.Hour),
	}

	html := `<div>מחיר בסיס ₪ 85,000</div>`
	client := &mockHTTPDoer{body: []byte(html), statusCode: 200}
	svc := NewService(store, client, slog.Default())

	bp, ok := svc.Lookup(context.Background(), 100, 2020)
	if !ok {
		t.Fatal("expected ok=true after re-fetch")
	}
	if bp != 85000 {
		t.Errorf("expected 85000, got %d", bp)
	}
}

func TestResetCycleCounter(t *testing.T) {
	svc := NewService(newMockStore(), &mockHTTPDoer{statusCode: 200, body: []byte(`מחיר בסיס ₪ 50,000`)}, slog.Default())

	for i := 0; i < maxFetchPerCycle; i++ {
		svc.Lookup(context.Background(), i+1, 2020)
		time.Sleep(fetchCooldown + time.Millisecond)
	}

	_, ok := svc.Lookup(context.Background(), 999, 2020)
	if ok {
		t.Error("expected ok=false after hitting cycle limit")
	}

	svc.ResetCycleCounter()
	time.Sleep(fetchCooldown + time.Millisecond)
	bp, ok := svc.Lookup(context.Background(), 999, 2020)
	if !ok {
		t.Error("expected ok=true after reset")
	}
	if bp != 50000 {
		t.Errorf("expected 50000, got %d", bp)
	}
}

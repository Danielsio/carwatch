package scheduler

import (
	"context"
	"testing"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type mockSearchStore struct {
	searches []storage.Search
}

func (m *mockSearchStore) CreateSearch(_ context.Context, s storage.Search) (int64, error) {
	s.ID = int64(len(m.searches) + 1)
	m.searches = append(m.searches, s)
	return s.ID, nil
}

func (m *mockSearchStore) ListSearches(_ context.Context, chatID int64) ([]storage.Search, error) {
	var result []storage.Search
	for _, s := range m.searches {
		if s.ChatID == chatID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSearchStore) GetSearch(_ context.Context, id int64) (*storage.Search, error) {
	for _, s := range m.searches {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, nil
}

func (m *mockSearchStore) DeleteSearch(_ context.Context, id int64, _ int64) error    { return nil }
func (m *mockSearchStore) SetSearchActive(_ context.Context, _ int64, _ bool) error   { return nil }
func (m *mockSearchStore) CountSearches(_ context.Context, _ int64) (int64, error)    { return 0, nil }
func (m *mockSearchStore) CountAllSearches(_ context.Context) (int64, error)          { return 0, nil }

func (m *mockSearchStore) ListAllActiveSearches(_ context.Context) ([]storage.Search, error) {
	var active []storage.Search
	for _, s := range m.searches {
		if s.Active {
			active = append(active, s)
		}
	}
	return active, nil
}

func TestRunMultiTenantCycle_NoSearches(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{SearchStore: ss})
	ctx := context.Background()

	err := s.runMultiTenantCycle(ctx)
	if err != nil {
		t.Fatalf("expected no error for empty searches, got: %v", err)
	}
}

func TestRunMultiTenantCycle_WithSearches(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
			{Token: "b", Manufacturer: "Mazda", Model: "3", Price: 80000, Year: 2019, EngineVolume: 1500},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1-mazda3", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{SearchStore: ss})
	ctx := context.Background()

	err := s.runMultiTenantCycle(ctx)
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification (engine filter should exclude 1500cc), got %d", len(n.messages))
	}
	if len(n.messages) > 0 && n.messages[0].count != 1 {
		t.Errorf("expected 1 listing in notification, got %d", n.messages[0].count)
	}
}

func TestRunMultiTenantCycle_SharedScraping(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 100000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 200000, Active: true},
			{ID: 2, ChatID: 200, Name: "user2", Manufacturer: 27, Model: 10332,
				YearMin: 2020, YearMax: 2026, PriceMax: 150000, Active: true},
		},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{SearchStore: ss})
	ctx := context.Background()

	err := s.runMultiTenantCycle(ctx)
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}

	f.mu.Lock()
	fetchCalls := f.calls
	f.mu.Unlock()

	if fetchCalls != 1 {
		t.Errorf("fetcher called %d times, want 1 (shared scraping)", fetchCalls)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 2 {
		t.Errorf("expected 2 notifications (one per user), got %d", len(n.messages))
	}
}

func TestRunMultiTenantCycle_FallbackToLegacy(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := &config.Config{
		Polling: config.PollingConfig{
			Interval: 1,
			Timezone: "UTC",
		},
		Searches: []config.SearchConfig{
			{Name: "test", Source: "yad2", Recipients: []string{"100"}},
		},
		Storage: config.StorageConfig{PruneAfter: 1},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{})
	ctx := context.Background()

	err := s.runMultiTenantCycle(ctx)
	if err != nil {
		t.Fatalf("fallback cycle: %v", err)
	}
}

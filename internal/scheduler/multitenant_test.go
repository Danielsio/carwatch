package scheduler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

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
	for i := range m.searches {
		if m.searches[i].ID == id {
			return &m.searches[i], nil
		}
	}
	return nil, nil
}

func (m *mockSearchStore) GetSearchBySeq(_ context.Context, chatID int64, seq int) (*storage.Search, error) {
	for i := range m.searches {
		if m.searches[i].ChatID == chatID && m.searches[i].UserSeq == seq {
			return &m.searches[i], nil
		}
	}
	return nil, nil
}

func (m *mockSearchStore) DeleteSearch(_ context.Context, id int64, _ int64) error    { return nil }
func (m *mockSearchStore) SetSearchActive(_ context.Context, _, _ int64, _ bool) error { return nil }
func (m *mockSearchStore) CountSearches(_ context.Context, _ int64) (int64, error)    { return 0, nil }
func (m *mockSearchStore) CountAllSearches(_ context.Context) (int64, error)          { return 0, nil }
func (m *mockSearchStore) UpdateSearch(_ context.Context, _ storage.Search) error     { return nil }

func (m *mockSearchStore) GetSearchByShareToken(_ context.Context, token string) (*storage.Search, error) {
	for i := range m.searches {
		if m.searches[i].ShareToken == token {
			return &m.searches[i], nil
		}
	}
	return nil, nil
}

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
			{ID: 1, ChatID: 100, Name: "user1-mazda3", Source: "yad2", Manufacturer: 27, Model: 10332,
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
			{ID: 1, ChatID: 100, Name: "user1", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 200000, Active: true},
			{ID: 2, ChatID: 200, Name: "user2", Source: "yad2", Manufacturer: 27, Model: 10332,
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

func TestProcessGroup_PriceDropNotification(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 89000, Year: 2021, EngineVolume: 2000, Km: 50000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	pt := newMockPriceTracker()
	pt.prices["a"] = 95000

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1-mazda3", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, Active: true},
		},
	}

	us := newMockUserStore()
	us.users[100] = &storage.User{
		ChatID: 100, Tier: "premium",
		TierExpires: time.Now().Add(30 * 24 * time.Hour),
		Language:    "he",
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		Prices:      pt,
		UserStore:   us,
	})
	ctx := context.Background()

	err := s.runMultiTenantCycle(ctx)
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.messages) != 0 {
		t.Errorf("expected 0 Notify calls for price drop, got %d", len(n.messages))
	}

	if len(n.rawMessages) != 1 {
		t.Fatalf("expected 1 NotifyRaw call for price drop, got %d", len(n.rawMessages))
	}

	if n.rawMessages[0].recipient != "100" {
		t.Errorf("expected recipient '100', got %q", n.rawMessages[0].recipient)
	}

	msg := n.rawMessages[0].message
	if !strings.Contains(msg, "ירידת מחיר") {
		t.Errorf("price drop message should contain 'ירידת מחיר', got:\n%s", msg)
	}
	if !strings.Contains(msg, "₪95,000") || !strings.Contains(msg, "₪89,000") {
		t.Errorf("message should contain old and new prices, got:\n%s", msg)
	}
}

type mockUserStore struct {
	users map[int64]*storage.User
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{users: make(map[int64]*storage.User)}
}

func (m *mockUserStore) UpsertUser(_ context.Context, chatID int64, username string) error {
	if _, ok := m.users[chatID]; !ok {
		m.users[chatID] = &storage.User{ChatID: chatID, Username: username, Tier: "free", Language: "he"}
	}
	return nil
}

func (m *mockUserStore) GetUser(_ context.Context, chatID int64) (*storage.User, error) {
	u, ok := m.users[chatID]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserStore) UpdateUserState(_ context.Context, _ int64, _ string, _ string) error {
	return nil
}
func (m *mockUserStore) ListActiveUsers(_ context.Context) ([]storage.User, error) { return nil, nil }
func (m *mockUserStore) SetUserActive(_ context.Context, _ int64, _ bool) error    { return nil }
func (m *mockUserStore) SetUserLanguage(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockUserStore) CountUsers(_ context.Context) (int64, error) {
	return int64(len(m.users)), nil
}
func (m *mockUserStore) SetUserTier(_ context.Context, chatID int64, tier string, expires time.Time) error {
	if u, ok := m.users[chatID]; ok {
		u.Tier = tier
		u.TierExpires = expires
	}
	return nil
}
func (m *mockUserStore) GrantTrial(_ context.Context, _ int64, _ time.Duration) error { return nil }
func (m *mockUserStore) ListExpiredPremium(_ context.Context) ([]storage.User, error) {
	return nil, nil
}
func (m *mockUserStore) GetUserByChannelID(_ context.Context, _, _ string) (*storage.User, error) {
	return nil, nil
}
func (m *mockUserStore) UpsertWhatsAppUser(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *mockUserStore) UpsertWebUser(_ context.Context, _, _ string) (int64, error) { return 0, nil }
func (m *mockUserStore) UpdateLastSeenAt(_ context.Context, _ int64) error { return nil }
func (m *mockUserStore) LinkTelegramToWeb(_ context.Context, _, _ int64) error { return nil }
func (m *mockUserStore) GetLinkedTelegramUser(_ context.Context, _ int64) (*storage.User, error) {
	return nil, nil
}

type digestItem struct {
	payload   string
	createdAt time.Time
}

func digestItems(payloads ...string) []digestItem {
	items := make([]digestItem, len(payloads))
	for i, p := range payloads {
		items[i] = digestItem{payload: p, createdAt: time.Now()}
	}
	return items
}

type mockDigestStore struct {
	mu       sync.Mutex
	modes    map[int64]struct{ mode, interval string }
	items    map[int64][]digestItem
	flushed  map[int64]time.Time
}

func newMockDigestStore() *mockDigestStore {
	return &mockDigestStore{
		modes:   make(map[int64]struct{ mode, interval string }),
		items:   make(map[int64][]digestItem),
		flushed: make(map[int64]time.Time),
	}
}

func (m *mockDigestStore) SetDigestMode(_ context.Context, chatID int64, mode string, interval string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modes[chatID] = struct{ mode, interval string }{mode, interval}
	return nil
}

func (m *mockDigestStore) GetDigestMode(_ context.Context, chatID int64) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.modes[chatID]; ok {
		return v.mode, v.interval, nil
	}
	return "instant", "6h", nil
}

func (m *mockDigestStore) AddDigestItem(_ context.Context, chatID int64, payload string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[chatID] = append(m.items[chatID], digestItem{payload: payload, createdAt: time.Now()})
	return nil
}


func (m *mockDigestStore) PeekDigest(_ context.Context, chatID int64) ([]string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now()
	var payloads []string
	for _, item := range m.items[chatID] {
		payloads = append(payloads, item.payload)
	}
	return payloads, cutoff, nil
}

func (m *mockDigestStore) AckDigest(_ context.Context, chatID int64, before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var kept []digestItem
	for _, item := range m.items[chatID] {
		if item.createdAt.After(before) {
			kept = append(kept, item)
		}
	}
	if len(kept) == 0 {
		delete(m.items, chatID)
	} else {
		m.items[chatID] = kept
	}
	m.flushed[chatID] = time.Now()
	return nil
}

func (m *mockDigestStore) PendingDigestUsers(_ context.Context) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var users []int64
	for chatID := range m.items {
		if len(m.items[chatID]) > 0 {
			users = append(users, chatID)
		}
	}
	return users, nil
}

func (m *mockDigestStore) DigestLastFlushed(_ context.Context, chatID int64) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushed[chatID], nil
}

func TestProcessGroup_DigestMode_StoresInsteadOfSending(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ds := newMockDigestStore()
	_ = ds.SetDigestMode(context.Background(), 100, "digest", "6h")
	// Set last flushed to now so processDigests won't flush immediately.
	ds.flushed[100] = time.Now()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1-mazda3", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		DigestStore: ds,
	})
	ctx := context.Background()

	err := s.runMultiTenantCycle(ctx)
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}

	// Should NOT have sent any direct Notify calls.
	n.mu.Lock()
	notifyCount := len(n.messages)
	rawCount := len(n.rawMessages)
	n.mu.Unlock()
	if notifyCount != 0 {
		t.Errorf("expected 0 direct Notify calls, got %d", notifyCount)
	}
	if rawCount != 0 {
		t.Errorf("expected 0 NotifyRaw calls (digest interval not elapsed), got %d", rawCount)
	}

	// Should have stored the item in the digest store.
	ds.mu.Lock()
	items := ds.items[100]
	ds.mu.Unlock()
	if len(items) != 1 {
		t.Errorf("expected 1 digest item, got %d", len(items))
	}
}

func TestProcessGroup_InstantMode_SendsDirectly(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ds := newMockDigestStore()
	// Default is instant, no need to set.

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1-mazda3", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		DigestStore: ds,
	})
	ctx := context.Background()

	err := s.runMultiTenantCycle(ctx)
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}

	// Should have sent directly.
	n.mu.Lock()
	notifyCount := len(n.messages)
	n.mu.Unlock()
	if notifyCount != 1 {
		t.Errorf("expected 1 direct notification, got %d", notifyCount)
	}

	// Digest store should be empty.
	ds.mu.Lock()
	items := ds.items[100]
	ds.mu.Unlock()
	if len(items) != 0 {
		t.Errorf("expected 0 digest items, got %d", len(items))
	}
}

func TestProcessDigests_FlushesWhenIntervalElapsed(t *testing.T) {
	n := &mockNotifier{}
	cfg := testConfig()

	ds := newMockDigestStore()
	_ = ds.SetDigestMode(context.Background(), 100, "digest", "1ms")
	ds.items[100] = digestItems("listing A", "listing B")
	// Set last flushed to epoch so interval has elapsed.
	ds.flushed[100] = time.Time{}

	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()

	ss := &mockSearchStore{}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		DigestStore: ds,
	})
	ctx := context.Background()

	s.processDigests(ctx)

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Fatalf("expected 1 digest notification, got %d", len(n.rawMessages))
	}

	msg := n.rawMessages[0].message
	if !strings.Contains(msg, "listing A") || !strings.Contains(msg, "listing B") {
		t.Errorf("digest message should contain items, got: %s", msg)
	}
	if !strings.Contains(msg, "סיכום יומי") {
		t.Errorf("digest message should contain header, got: %s", msg)
	}
}

func TestProcessDigests_SkipsWhenIntervalNotElapsed(t *testing.T) {
	n := &mockNotifier{}
	cfg := testConfig()

	ds := newMockDigestStore()
	_ = ds.SetDigestMode(context.Background(), 100, "digest", "24h")
	ds.items[100] = digestItems("listing A")
	ds.flushed[100] = time.Now() // Just flushed.

	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	ss := &mockSearchStore{}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		DigestStore: ds,
	})
	ctx := context.Background()

	s.processDigests(ctx)

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 0 {
		t.Errorf("expected 0 digest notifications (interval not elapsed), got %d", len(n.rawMessages))
	}
}

func TestProcessDigests_FlushesImmediatelyWhenSwitchedToInstant(t *testing.T) {
	n := &mockNotifier{}
	cfg := testConfig()

	ds := newMockDigestStore()
	// User has pending items but switched back to instant.
	_ = ds.SetDigestMode(context.Background(), 100, "instant", "6h")
	ds.items[100] = digestItems("leftover listing")

	f := &mockFetcher{listings: []model.RawListing{}}
	d := newMockDedup()
	ss := &mockSearchStore{}

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		DigestStore: ds,
	})
	ctx := context.Background()

	s.processDigests(ctx)

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Fatalf("expected 1 digest notification (flush on mode switch), got %d", len(n.rawMessages))
	}
	if !strings.Contains(n.rawMessages[0].message, "leftover listing") {
		t.Errorf("flushed digest should contain pending items")
	}
}

// --- mockHiddenStore ---

type mockHiddenStore struct {
	mu     sync.Mutex
	hidden map[int64]map[string]bool
	err    error
}

func newMockHiddenStore() *mockHiddenStore {
	return &mockHiddenStore{hidden: make(map[int64]map[string]bool)}
}

func (m *mockHiddenStore) HideListing(_ context.Context, chatID int64, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hidden[chatID] == nil {
		m.hidden[chatID] = make(map[string]bool)
	}
	m.hidden[chatID][token] = true
	return nil
}

func (m *mockHiddenStore) IsHidden(_ context.Context, chatID int64, token string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hidden[chatID][token], m.err
}

func (m *mockHiddenStore) ListHiddenTokens(_ context.Context, chatID int64) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	result := make(map[string]bool)
	for k, v := range m.hidden[chatID] {
		result[k] = v
	}
	return result, nil
}

func (m *mockHiddenStore) ListHidden(_ context.Context, _ int64, _, _ int) ([]string, error) {
	return nil, nil
}

func (m *mockHiddenStore) CountHidden(_ context.Context, chatID int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.hidden[chatID])), nil
}

func (m *mockHiddenStore) UnhideListing(_ context.Context, chatID int64, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.hidden[chatID]; ok {
		delete(h, token)
	}
	return nil
}

func (m *mockHiddenStore) ClearHidden(_ context.Context, chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hidden, chatID)
	return nil
}

// --- Hidden listing filtering tests (#213) ---

func TestRunMultiTenantCycle_HiddenListingFiltered(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "visible", Price: 90000, Year: 2020, EngineVolume: 2000},
			{Token: "hidden1", Price: 80000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, Active: true},
		},
	}

	hs := newMockHiddenStore()
	_ = hs.HideListing(context.Background(), 100, "hidden1")

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		HiddenStore: hs,
	})
	ctx := context.Background()

	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(n.messages))
	}
	if n.messages[0].count != 1 {
		t.Errorf("expected 1 listing (hidden1 filtered), got %d", n.messages[0].count)
	}
}

func TestRunMultiTenantCycle_HiddenCrossUserIsolation(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tokA", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "user1", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, Active: true},
			{ID: 2, ChatID: 200, Name: "user2", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, Active: true},
		},
	}

	hs := newMockHiddenStore()
	_ = hs.HideListing(context.Background(), 100, "tokA")
	// User 200 has NOT hidden tokA

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		HiddenStore: hs,
	})
	ctx := context.Background()

	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// User 100 should get 0 listings (tokA hidden), user 200 should get 1
	if len(n.messages) != 1 {
		t.Fatalf("expected 1 notification (only user 200), got %d", len(n.messages))
	}
	if n.messages[0].recipient != "200" {
		t.Errorf("expected notification for user 200, got %q", n.messages[0].recipient)
	}
}

func TestRunMultiTenantCycle_HiddenStoreError(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "tokA", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	cfg := testConfig()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, Active: true},
		},
	}

	hs := newMockHiddenStore()
	hs.err = errors.New("db error")

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		HiddenStore: hs,
	})
	ctx := context.Background()

	if err := s.runMultiTenantCycle(ctx); err != nil {
		t.Fatalf("cycle should succeed despite hidden store error: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	// Listing should still be delivered since hidden store error is non-fatal
	if len(n.messages) != 1 {
		t.Fatalf("expected 1 notification (hidden check failed gracefully), got %d", len(n.messages))
	}
}


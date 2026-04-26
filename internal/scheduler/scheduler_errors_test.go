package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

// --- Error-returning mock variants ---

type errDedup struct {
	mockDedup
	claimErr   error
	releaseErr error
	pruneErr   error
}

func newErrDedup() *errDedup {
	return &errDedup{mockDedup: mockDedup{seen: make(map[dedupKey]bool)}}
}

func (m *errDedup) ClaimNew(ctx context.Context, token string, chatID int64, searchID int64) (bool, error) {
	if m.claimErr != nil {
		return false, m.claimErr
	}
	return m.mockDedup.ClaimNew(ctx, token, chatID, searchID)
}

func (m *errDedup) ReleaseClaim(ctx context.Context, token string, chatID int64) error {
	if m.releaseErr != nil {
		return m.releaseErr
	}
	return m.mockDedup.ReleaseClaim(ctx, token, chatID)
}

func (m *errDedup) Prune(_ context.Context, _ time.Duration) (int64, error) {
	if m.pruneErr != nil {
		return 0, m.pruneErr
	}
	return 0, nil
}

type errPriceTracker struct {
	mockPriceTracker
	err error
}

func newErrPriceTracker() *errPriceTracker {
	return &errPriceTracker{mockPriceTracker: mockPriceTracker{prices: make(map[string]int)}}
}

func (m *errPriceTracker) RecordPrice(ctx context.Context, token string, price int) (int, bool, error) {
	if m.err != nil {
		return 0, false, m.err
	}
	return m.mockPriceTracker.RecordPrice(ctx, token, price)
}

type errSearchStore struct {
	mockSearchStore
	listErr error
}

func (m *errSearchStore) ListAllActiveSearches(_ context.Context) ([]storage.Search, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.mockSearchStore.ListAllActiveSearches(context.Background())
}

type errDigestStore struct {
	mockDigestStore
	getModeErr      error
	addItemErr      error
	flushErr        error
	ackErr          error
	pendingErr      error
	lastFlushedErr  error
}

func newErrDigestStore() *errDigestStore {
	return &errDigestStore{
		mockDigestStore: mockDigestStore{
			modes:   make(map[int64]struct{ mode, interval string }),
			items:   make(map[int64][]digestItem),
			flushed: make(map[int64]time.Time),
		},
	}
}

func (m *errDigestStore) GetDigestMode(ctx context.Context, chatID int64) (string, string, error) {
	if m.getModeErr != nil {
		return "", "", m.getModeErr
	}
	return m.mockDigestStore.GetDigestMode(ctx, chatID)
}

func (m *errDigestStore) AddDigestItem(ctx context.Context, chatID int64, payload string) error {
	if m.addItemErr != nil {
		return m.addItemErr
	}
	return m.mockDigestStore.AddDigestItem(ctx, chatID, payload)
}


func (m *errDigestStore) PeekDigest(ctx context.Context, chatID int64) ([]string, time.Time, error) {
	if m.flushErr != nil {
		return nil, time.Time{}, m.flushErr
	}
	return m.mockDigestStore.PeekDigest(ctx, chatID)
}

func (m *errDigestStore) AckDigest(ctx context.Context, chatID int64, before time.Time) error {
	if m.ackErr != nil {
		return m.ackErr
	}
	return m.mockDigestStore.AckDigest(ctx, chatID, before)
}

func (m *errDigestStore) PendingDigestUsers(_ context.Context) ([]int64, error) {
	if m.pendingErr != nil {
		return nil, m.pendingErr
	}
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

func (m *errDigestStore) DigestLastFlushed(ctx context.Context, chatID int64) (time.Time, error) {
	if m.lastFlushedErr != nil {
		return time.Time{}, m.lastFlushedErr
	}
	return m.mockDigestStore.DigestLastFlushed(ctx, chatID)
}

type errNotifier struct {
	mockNotifier
	rawErr error
}

func (m *errNotifier) NotifyRaw(_ context.Context, recipient string, message string) error {
	if m.rawErr != nil {
		return m.rawErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rawMessages = append(m.rawMessages, rawNotifyCall{recipient: recipient, message: message})
	return nil
}

type errNotificationQueue struct {
	mockNotificationQueue
	pendingErr  error
	enqueueErr  error
	ackErr      error
}

func (m *errNotificationQueue) PendingNotifications(_ context.Context) ([]storage.PendingNotification, error) {
	if m.pendingErr != nil {
		return nil, m.pendingErr
	}
	return m.pending, nil
}

func (m *errNotificationQueue) EnqueueNotification(_ context.Context, _, _, _ string) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	return nil
}

func (m *errNotificationQueue) AckNotification(_ context.Context, id int64) error {
	if m.ackErr != nil {
		return m.ackErr
	}
	if m.acked == nil {
		m.acked = make(map[int64]bool)
	}
	m.acked[id] = true
	return nil
}

type mockCatalogIngester struct {
	mu       sync.Mutex
	ingested int
	flushed  int
}

func (m *mockCatalogIngester) Ingest(_ context.Context, _ int, _ string, _ int, _ string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ingested++
}

func (m *mockCatalogIngester) Flush(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushed++
}

// --- Tests ---

func TestProcessGroup_ClaimNewError(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newErrDedup()
	d.claimErr = errors.New("db locked")
	n := &mockNotifier{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{SearchStore: ss})

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle should succeed even with claim errors: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 0 {
		t.Errorf("expected 0 notifications when claim fails, got %d", len(n.messages))
	}
}

func TestProcessGroup_RecordPriceError(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	pt := newErrPriceTracker()
	pt.err = errors.New("price db error")

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		Prices:      pt,
	})

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle should succeed despite price tracking error: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("listing should still be notified despite price error, got %d notifications", len(n.messages))
	}
}

func TestProcessGroup_DigestModeError(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}

	ds := newErrDigestStore()
	ds.getModeErr = errors.New("digest db error")

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		DigestStore: ds,
	})

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle should succeed despite digest mode error: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("should fall back to instant mode, got %d notifications", len(n.messages))
	}
}

func TestProcessGroup_DigestAddItemError_ReleasesClaims(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}

	ds := newErrDigestStore()
	_ = ds.SetDigestMode(context.Background(), 100, "digest", "6h")
	ds.flushed[100] = time.Now()
	ds.addItemErr = errors.New("write failed")

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		DigestStore: ds,
	})

	_ = s.runMultiTenantCycle(context.Background())

	d.mu.Lock()
	_, claimed := d.seen[dedupKey{"a", 100}]
	d.mu.Unlock()
	if claimed {
		t.Error("claim should be released when digest add fails")
	}
}

func TestProcessGroup_PriceDropInDigestMode(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 89000, Year: 2021, EngineVolume: 2000, Km: 50000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}

	pt := newMockPriceTracker()
	pt.prices["a"] = 95000

	ds := newMockDigestStore()
	_ = ds.SetDigestMode(context.Background(), 100, "digest", "6h")
	ds.flushed[100] = time.Now()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		Prices:      pt,
		DigestStore: ds,
	})

	_ = s.runMultiTenantCycle(context.Background())

	n.mu.Lock()
	rawCount := len(n.rawMessages)
	n.mu.Unlock()
	if rawCount != 0 {
		t.Errorf("price drop should go to digest, not instant. got %d raw messages", rawCount)
	}

	ds.mu.Lock()
	items := ds.items[100]
	ds.mu.Unlock()
	if len(items) != 1 {
		t.Errorf("expected 1 digest item for price drop, got %d", len(items))
	}
}

func TestProcessGroup_NotifyFails_EnqueuesAndKeepsClaim(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{err: errors.New("telegram down")}
	q := &errNotificationQueue{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		Queue:       q,
	})

	_ = s.runMultiTenantCycle(context.Background())

	d.mu.Lock()
	_, claimed := d.seen[dedupKey{"a", 100}]
	d.mu.Unlock()
	if !claimed {
		t.Error("claim should be kept when enqueue succeeds")
	}
}

func TestProcessGroup_NotifyFails_EnqueueFails_ReleasesClaim(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{err: errors.New("telegram down")}
	q := &errNotificationQueue{enqueueErr: errors.New("queue full")}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		Queue:       q,
	})

	_ = s.runMultiTenantCycle(context.Background())

	d.mu.Lock()
	_, claimed := d.seen[dedupKey{"a", 100}]
	d.mu.Unlock()
	if claimed {
		t.Error("claim should be released when both notify and enqueue fail")
	}
}

func TestRunMultiTenantCycle_SearchStoreError(t *testing.T) {
	ss := &errSearchStore{listErr: errors.New("db connection lost")}
	s, _ := NewWithOptions(testConfig(), nil, nil, nil, testLogger(), Options{SearchStore: ss})

	err := s.runMultiTenantCycle(context.Background())
	if err == nil {
		t.Fatal("expected error when search store fails")
	}
	if !errors.Is(err, ss.listErr) {
		t.Errorf("expected wrapped db error, got: %v", err)
	}
}

func TestRunMultiTenantCycle_PruneError(t *testing.T) {
	f := &mockFetcher{listings: []model.RawListing{}}
	d := newErrDedup()
	d.pruneErr = errors.New("prune failed")
	n := &mockNotifier{}
	h := health.New()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332, Active: true},
		},
	}
	cfg := testConfig()
	cfg.Storage.PruneAfter = 1 * time.Hour

	s, _ := NewWithOptions(cfg, f, d, n, testLogger(), Options{
		SearchStore: ss,
		Observer:    h,
	})
	s.lastPruneTime = time.Time{} // force prune

	err := s.runMultiTenantCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle should succeed despite prune error: %v", err)
	}
}

func TestRetryPending_PendingNotificationsError(t *testing.T) {
	q := &errNotificationQueue{
		pendingErr: errors.New("db error"),
	}
	s, _ := NewWithOptions(testConfig(), nil, nil, &mockNotifier{}, testLogger(), Options{Queue: q})
	s.retryPending(context.Background())
}

func TestRetryPending_NotifyRawFails(t *testing.T) {
	n := &errNotifier{rawErr: errors.New("send failed")}
	q := &errNotificationQueue{
		mockNotificationQueue: mockNotificationQueue{
			pending: []storage.PendingNotification{
				{ID: 1, Recipient: "123", Payload: "test"},
			},
		},
	}

	s, _ := NewWithOptions(testConfig(), nil, nil, n, testLogger(), Options{Queue: q})
	s.retryPending(context.Background())

	if q.acked != nil && q.acked[1] {
		t.Error("should not ack notification when retry fails")
	}
}

func TestRetryPending_AckFails(t *testing.T) {
	n := &mockNotifier{}
	q := &errNotificationQueue{
		mockNotificationQueue: mockNotificationQueue{
			pending: []storage.PendingNotification{
				{ID: 1, Recipient: "123", Payload: "test"},
			},
		},
		ackErr: errors.New("ack failed"),
	}

	s, _ := NewWithOptions(testConfig(), nil, nil, n, testLogger(), Options{Queue: q})
	s.retryPending(context.Background())

	if len(n.rawMessages) != 1 {
		t.Errorf("message should still be sent, got %d", len(n.rawMessages))
	}
}

func TestProcessDigests_PendingUsersError(t *testing.T) {
	ds := newErrDigestStore()
	ds.pendingErr = errors.New("db error")

	s, _ := NewWithOptions(testConfig(), nil, nil, &mockNotifier{}, testLogger(), Options{DigestStore: ds})
	s.processDigests(context.Background())
}

func TestProcessDigests_GetDigestModeError(t *testing.T) {
	ds := newErrDigestStore()
	ds.items[100] = digestItems("item1")
	ds.getModeErr = errors.New("mode error")

	s, _ := NewWithOptions(testConfig(), nil, nil, &mockNotifier{}, testLogger(), Options{DigestStore: ds})
	s.processDigests(context.Background())
}

func TestProcessDigests_LastFlushedError(t *testing.T) {
	n := &mockNotifier{}
	ds := newErrDigestStore()
	_ = ds.SetDigestMode(context.Background(), 100, "digest", "6h")
	ds.items[100] = digestItems("item1")
	ds.lastFlushedErr = errors.New("flushed error")

	s, _ := NewWithOptions(testConfig(), nil, nil, n, testLogger(), Options{DigestStore: ds})
	s.processDigests(context.Background())

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 0 {
		t.Errorf("should not send digest on lastFlushed error, got %d", len(n.rawMessages))
	}
}

func TestProcessDigests_InvalidIntervalDefaultsTo6h(t *testing.T) {
	n := &mockNotifier{}
	ds := newErrDigestStore()
	_ = ds.SetDigestMode(context.Background(), 100, "digest", "not-a-duration")
	ds.items[100] = digestItems("item1")
	ds.flushed[100] = time.Now().Add(-7 * time.Hour) // older than 6h default

	s, _ := NewWithOptions(testConfig(), nil, nil, n, testLogger(), Options{DigestStore: ds})
	s.processDigests(context.Background())

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Errorf("should flush with default 6h interval, got %d messages", len(n.rawMessages))
	}
}

func TestFlushAndSendDigest_FlushError(t *testing.T) {
	n := &mockNotifier{}
	ds := newErrDigestStore()
	ds.items[100] = digestItems("item1")
	ds.flushErr = errors.New("flush failed")

	s, _ := NewWithOptions(testConfig(), nil, nil, n, testLogger(), Options{DigestStore: ds})
	s.flushAndSendDigest(context.Background(), 100)

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 0 {
		t.Errorf("should not send when flush fails, got %d messages", len(n.rawMessages))
	}
}

func TestFlushAndSendDigest_SendError(t *testing.T) {
	n := &errNotifier{rawErr: errors.New("send failed")}
	ds := newMockDigestStore()
	ds.items[100] = digestItems("item1")

	s, _ := NewWithOptions(testConfig(), nil, nil, n, testLogger(), Options{DigestStore: ds})
	s.flushAndSendDigest(context.Background(), 100)
}

func TestRunMultiTenantCycle_CatalogIngester(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", ManufacturerID: 27, Manufacturer: "Mazda", ModelID: 10332, Model: "3",
				Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	ci := &mockCatalogIngester{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore:     ss,
		CatalogIngester: ci,
	})

	_ = s.runMultiTenantCycle(context.Background())

	ci.mu.Lock()
	defer ci.mu.Unlock()
	if ci.ingested != 1 {
		t.Errorf("expected 1 ingest call, got %d", ci.ingested)
	}
	if ci.flushed != 1 {
		t.Errorf("expected 1 flush call, got %d", ci.flushed)
	}
}

func TestRunMultiTenantCycle_HealthRecording(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}
	h := health.New()

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{
		SearchStore: ss,
		Observer:    h,
	})

	_ = s.runMultiTenantCycle(context.Background())

	snap := h.Snapshot()
	if snap["listings_found"] != int64(1) {
		t.Errorf("expected 1 listing found, got %v", snap["listings_found"])
	}
	if snap["notifications_sent"] != int64(1) {
		t.Errorf("expected 1 notification sent, got %v", snap["notifications_sent"])
	}
}

func TestReloadConfig_ValidConfig(t *testing.T) {
	cfg := testConfig()
	s, _ := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{
		ConfigPath: "testdata/valid_config.yaml",
	})

	origCfg := s.cfg
	s.reloadConfig()
	// Config may or may not change depending on testdata existence.
	// The key test is that it doesn't panic.
	_ = origCfg
}

func TestReloadConfig_InvalidTimezone(t *testing.T) {
	cfg := testConfig()
	s, _ := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{
		ConfigPath: "testdata/bad_timezone_config.yaml",
	})

	origLoc := s.loc
	s.reloadConfig()
	if s.loc != origLoc {
		t.Error("timezone should not change on failed reload")
	}
}

func TestProcessGroup_FiltersCorrectly(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "cheap", Price: 50000, Year: 2020, EngineVolume: 2000},
			{Token: "expensive", Price: 200000, Year: 2020, EngineVolume: 2000},
			{Token: "old", Price: 90000, Year: 2010, EngineVolume: 2000},
			{Token: "future", Price: 90000, Year: 2030, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{SearchStore: ss})
	_ = s.runMultiTenantCycle(context.Background())

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(n.messages))
	}
	if n.messages[0].count != 1 {
		t.Errorf("only 'cheap' should match, got %d listings", n.messages[0].count)
	}
}

func TestProcessGroup_PriceMaxZero_NoFilter(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 999999, Year: 2020, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 2024, PriceMax: 0, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{SearchStore: ss})
	_ = s.runMultiTenantCycle(context.Background())

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("PriceMax=0 should not filter, got %d notifications", len(n.messages))
	}
}

func TestProcessGroup_YearMaxZero_NoUpperBound(t *testing.T) {
	f := &mockFetcher{
		listings: []model.RawListing{
			{Token: "a", Price: 90000, Year: 2030, EngineVolume: 2000},
		},
	}
	d := newMockDedup()
	n := &mockNotifier{}

	ss := &mockSearchStore{
		searches: []storage.Search{
			{ID: 1, ChatID: 100, Name: "test", Source: "yad2", Manufacturer: 27, Model: 10332,
				YearMin: 2018, YearMax: 0, PriceMax: 150000, EngineMinCC: 1800, Active: true},
		},
	}

	s, _ := NewWithOptions(testConfig(), f, d, n, testLogger(), Options{SearchStore: ss})
	_ = s.runMultiTenantCycle(context.Background())

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("YearMax=0 should not filter, got %d notifications", len(n.messages))
	}
}

func TestFlushAndSendDigest_AckFails_LogsDistinctiveError(t *testing.T) {
	n := &mockNotifier{}
	ds := newErrDigestStore()
	ds.items[100] = digestItems("item1", "item2")
	ds.ackErr = errors.New("ack failed")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError}))

	h := health.New()
	s, _ := NewWithOptions(testConfig(), nil, nil, n, logger, Options{
		DigestStore: ds,
		Observer:    h,
	})

	s.flushAndSendDigest(context.Background(), 100)

	// Notification should still have been sent despite ack failure.
	n.mu.Lock()
	sentCount := len(n.rawMessages)
	n.mu.Unlock()
	if sentCount != 1 {
		t.Errorf("expected 1 message sent, got %d", sentCount)
	}

	// The distinctive error message should appear in the log.
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "digest ack failed after successful send, items may be resent") {
		t.Errorf("expected distinctive ack-failure log message, got: %s", logOutput)
	}
}

func TestSendDailyDigest_LastSentUpdateFails_LogsDistinctiveError(t *testing.T) {
	n := &mockNotifier{}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError}))

	dds := &errDailyDigestStore{
		stats: []storage.DailySearchStats{
			{SearchName: "test", NewCount: 5, AvgPrice: 100000, BestPrice: 90000},
		},
		updateLastSentErr: errors.New("db write failed"),
	}

	s, _ := NewWithOptions(testConfig(), nil, nil, n, logger, Options{
		DailyDigestStore: dds,
	})

	s.sendDailyDigest(context.Background(), 100)

	// Notification should still have been sent despite last-sent update failure.
	n.mu.Lock()
	sentCount := len(n.rawMessages)
	n.mu.Unlock()
	if sentCount != 1 {
		t.Errorf("expected 1 message sent, got %d", sentCount)
	}

	// The distinctive error message should appear in the log.
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "daily digest last-sent update failed after successful send, digest may be resent") {
		t.Errorf("expected distinctive last-sent-failure log message, got: %s", logOutput)
	}
}

// errDailyDigestStore is a mock DailyDigestStore that can inject errors.
type errDailyDigestStore struct {
	stats             []storage.DailySearchStats
	updateLastSentErr error
}

func (m *errDailyDigestStore) SetDailyDigest(_ context.Context, _ int64, _ bool, _ string) error {
	return nil
}

func (m *errDailyDigestStore) GetDailyDigest(_ context.Context, _ int64) (bool, string, time.Time, error) {
	return true, "08:00", time.Time{}, nil
}

func (m *errDailyDigestStore) UpdateDailyDigestLastSent(_ context.Context, _ int64) error {
	return m.updateLastSentErr
}

func (m *errDailyDigestStore) ListDailyDigestUsers(_ context.Context) ([]storage.DailyDigestUser, error) {
	return nil, nil
}

func (m *errDailyDigestStore) DailyStats(_ context.Context, _ int64) ([]storage.DailySearchStats, error) {
	return m.stats, nil
}

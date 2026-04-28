package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func TestFetcherForSource_WithFactory(t *testing.T) {
	factory := fetcher.NewFactory()
	yad2Fetcher := &mockFetcher{listings: []model.RawListing{{Token: "yad2-listing"}}}
	winwinFetcher := &mockFetcher{listings: []model.RawListing{{Token: "winwin-listing"}}}
	defaultFetcher := &mockFetcher{listings: []model.RawListing{{Token: "default"}}}

	factory.Register("yad2", yad2Fetcher)
	factory.Register("winwin", winwinFetcher)

	cfg := testConfig()
	s, err := NewWithOptions(cfg, defaultFetcher, newMockDedup(), &mockNotifier{}, testLogger(), Options{
		FetcherFactory: factory,
	})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	f := s.fetcherForSource("yad2")
	if f != yad2Fetcher {
		t.Error("expected yad2 fetcher from factory")
	}

	f = s.fetcherForSource("winwin")
	if f != winwinFetcher {
		t.Error("expected winwin fetcher from factory")
	}

	f = s.fetcherForSource("unknown")
	if f != defaultFetcher {
		t.Error("expected default fetcher for unknown source")
	}
}

func TestFetcherForSource_NilFactory(t *testing.T) {
	defaultFetcher := &mockFetcher{}
	cfg := testConfig()
	s, _ := New(cfg, defaultFetcher, newMockDedup(), &mockNotifier{}, testLogger(), nil)

	f := s.fetcherForSource("yad2")
	if f != defaultFetcher {
		t.Error("expected default fetcher when factory is nil")
	}
}

func TestDurationUntilActiveStart(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.ActiveHours = &config.ActiveHours{
		Start: "08:00",
		End:   "22:00",
	}

	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)

	dur := s.durationUntilActiveStart()
	if dur <= 0 || dur > 24*time.Hour {
		t.Errorf("duration should be between 0 and 24h, got %v", dur)
	}
}

func TestDurationUntilActiveStart_NoActiveHours(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.ActiveHours = nil

	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)

	dur := s.durationUntilActiveStart()
	if dur != 0 {
		t.Errorf("expected 0 duration when no active hours, got %v", dur)
	}
}

func TestParseTimeOfDayOrZero(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"08:00", 8 * 60},
		{"22:30", 22*60 + 30},
		{"00:00", 0},
		{"23:59", 23*60 + 59},
		{"invalid", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseTimeOfDayOrZero(tt.input)
		if got != tt.want {
			t.Errorf("parseTimeOfDayOrZero(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestReloadConfig_EmptyPath(t *testing.T) {
	cfg := testConfig()
	s, _ := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{ConfigPath: ""})
	s.reloadConfig()
}

func TestReloadConfig_InvalidPath(t *testing.T) {
	cfg := testConfig()
	s, _ := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{ConfigPath: "/nonexistent/config.yaml"})
	s.reloadConfig()
	if s.cfg != cfg {
		t.Error("config should not change on failed reload")
	}
}

func TestRetryPending_NilQueue(t *testing.T) {
	cfg := testConfig()
	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)
	s.retryPending(context.Background())
}

func TestRetryPending_EmptyQueue(t *testing.T) {
	cfg := testConfig()
	queue := &mockNotificationQueue{}
	s, _ := NewWithOptions(cfg, nil, nil, &mockNotifier{}, testLogger(), Options{Queue: queue})
	s.retryPending(context.Background())
}

func TestRetryPending_WithPending(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifier{}
	queue := &mockNotificationQueue{
		pending: []storage.PendingNotification{
			{ID: 1, Recipient: "+972501234567", Payload: "test message"},
		},
	}
	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{Queue: queue})
	s.retryPending(context.Background())

	if len(n.rawMessages) != 1 {
		t.Errorf("expected 1 retried message, got %d", len(n.rawMessages))
	}
	if !queue.acked[1] {
		t.Error("expected notification to be acknowledged")
	}
}

func TestNewWithOptions_HealthStatus(t *testing.T) {
	cfg := testConfig()
	h := health.New()
	s, err := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{Observer: h})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	if s.observer != h {
		t.Error("observer not set")
	}
}

func TestNewWithOptions_InvalidTimezone(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.Timezone = "Invalid/Zone"
	_, err := New(cfg, nil, nil, nil, testLogger(), nil)
	if err == nil {
		t.Error("expected error for invalid timezone")
	}
}

func TestNextDelay_WithJitter(t *testing.T) {
	cfg := testConfig()
	cfg.Polling.Interval = 10 * time.Minute
	cfg.Polling.Jitter = 2 * time.Minute

	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)

	for range 10 {
		delay := s.nextDelay()
		if delay < 8*time.Minute || delay > 12*time.Minute {
			t.Errorf("delay with jitter should be 8-12m, got %v", delay)
		}
	}
}

type mockNotificationQueue struct {
	pending []storage.PendingNotification
	acked   map[int64]bool
}

func (m *mockNotificationQueue) EnqueueNotification(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockNotificationQueue) PendingNotifications(_ context.Context) ([]storage.PendingNotification, error) {
	return m.pending, nil
}

func (m *mockNotificationQueue) AckNotification(_ context.Context, id int64) error {
	if m.acked == nil {
		m.acked = make(map[int64]bool)
	}
	m.acked[id] = true
	return nil
}

func (m *mockNotificationQueue) PruneNotifications(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

// --- mockDailyDigestStore ---

type mockDailyDigestStore struct {
	mu       sync.Mutex
	digests  map[int64]struct{ enabled bool; digestTime string; lastSent time.Time }
	stats    map[int64][]storage.DailySearchStats
	updated  map[int64]bool
	listErr  error
	statsErr error
}

func newMockDailyDigestStore() *mockDailyDigestStore {
	return &mockDailyDigestStore{
		digests: make(map[int64]struct{ enabled bool; digestTime string; lastSent time.Time }),
		stats:   make(map[int64][]storage.DailySearchStats),
		updated: make(map[int64]bool),
	}
}

func (m *mockDailyDigestStore) SetDailyDigest(_ context.Context, chatID int64, enabled bool, digestTime string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.digests[chatID] = struct{ enabled bool; digestTime string; lastSent time.Time }{enabled, digestTime, m.digests[chatID].lastSent}
	return nil
}

func (m *mockDailyDigestStore) GetDailyDigest(_ context.Context, chatID int64) (bool, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.digests[chatID]
	if !ok {
		return false, "09:00", time.Time{}, nil
	}
	return d.enabled, d.digestTime, d.lastSent, nil
}

func (m *mockDailyDigestStore) UpdateDailyDigestLastSent(_ context.Context, chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updated[chatID] = true
	return nil
}

func (m *mockDailyDigestStore) ListDailyDigestUsers(_ context.Context) ([]storage.DailyDigestUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	var users []storage.DailyDigestUser
	for chatID, d := range m.digests {
		if d.enabled {
			users = append(users, storage.DailyDigestUser{ChatID: chatID, DigestTime: d.digestTime, LastSent: d.lastSent})
		}
	}
	return users, nil
}

func (m *mockDailyDigestStore) DailyStats(_ context.Context, chatID int64) ([]storage.DailySearchStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.stats[chatID], nil
}

// --- TriggerPoll tests ---

func TestTriggerPoll_SendsOnChannel(t *testing.T) {
	cfg := testConfig()
	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)

	s.TriggerPoll()

	select {
	case <-s.triggerCh:
	default:
		t.Error("expected value on trigger channel")
	}
}

func TestTriggerPoll_DoesNotBlock(t *testing.T) {
	cfg := testConfig()
	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)

	s.TriggerPoll()
	s.TriggerPoll()

	select {
	case <-s.triggerCh:
	default:
		t.Error("expected value on trigger channel")
	}
}

// --- processExpiredPremium tests ---

func TestProcessExpiredPremium_IsNoOp(t *testing.T) {
	cfg := testConfig()
	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)
	s.processExpiredPremium(context.Background())
}

// --- deactivateExcessSearches tests ---

func TestDeactivateExcessSearches_NilSearchStore(t *testing.T) {
	cfg := testConfig()
	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)
	s.deactivateExcessSearches(context.Background(), 100, 1)
}

func TestDeactivateExcessSearches_NoExcess(t *testing.T) {
	cfg := testConfig()
	ss := &mockSearchStoreWithTracking{
		mockSearchStore: &mockSearchStore{
			searches: []storage.Search{
				{ID: 1, ChatID: 100, Active: true},
			},
		},
	}
	s, _ := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{SearchStore: ss})

	s.deactivateExcessSearches(context.Background(), 100, 1)

	if len(ss.deactivated) != 0 {
		t.Error("should not deactivate when at or below limit")
	}
}

func TestDeactivateExcessSearches_DeactivatesExcess(t *testing.T) {
	cfg := testConfig()
	ss := &mockSearchStoreWithTracking{
		mockSearchStore: &mockSearchStore{
			searches: []storage.Search{
				{ID: 1, ChatID: 100, Active: true},
				{ID: 2, ChatID: 100, Active: true},
				{ID: 3, ChatID: 100, Active: true},
			},
		},
	}
	s, _ := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{SearchStore: ss})

	s.deactivateExcessSearches(context.Background(), 100, 1)

	if len(ss.deactivated) != 2 {
		t.Errorf("expected 2 deactivated, got %d", len(ss.deactivated))
	}
}

type mockSearchStoreWithTracking struct {
	*mockSearchStore
	deactivated []int64
}

func (m *mockSearchStoreWithTracking) SetSearchActive(_ context.Context, id int64, _ int64, active bool) error {
	if !active {
		m.deactivated = append(m.deactivated, id)
	}
	return nil
}

// --- processDailyDigests tests ---

func TestProcessDailyDigests_NilStore(t *testing.T) {
	cfg := testConfig()
	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)
	s.processDailyDigests(context.Background())
}

func TestProcessDailyDigests_ListError(t *testing.T) {
	cfg := testConfig()
	dds := newMockDailyDigestStore()
	dds.listErr = errors.New("db error")
	n := &mockNotifier{}

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.processDailyDigests(context.Background())

	if len(n.rawMessages) != 0 {
		t.Error("should not send anything on list error")
	}
}

func TestProcessDailyDigests_SendsWhenTimeMatches(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifier{}
	dds := newMockDailyDigestStore()

	now := time.Now().In(time.UTC)
	digestTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	dds.digests[100] = struct{ enabled bool; digestTime string; lastSent time.Time }{
		true, digestTime, time.Time{},
	}
	dds.stats[100] = []storage.DailySearchStats{
		{SearchName: "test", NewCount: 5, AvgPrice: 100000, BestPrice: 80000},
	}

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.processDailyDigests(context.Background())

	if len(n.rawMessages) != 1 {
		t.Errorf("expected 1 digest sent, got %d", len(n.rawMessages))
	}
	if !dds.updated[100] {
		t.Error("expected last sent to be updated")
	}
}

func TestProcessDailyDigests_SkipsOutsideWindow(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifier{}
	dds := newMockDailyDigestStore()

	now := time.Now().In(time.UTC)
	farHour := (now.Hour() + 6) % 24
	digestTime := fmt.Sprintf("%02d:%02d", farHour, now.Minute())

	dds.digests[100] = struct{ enabled bool; digestTime string; lastSent time.Time }{
		true, digestTime, time.Time{},
	}

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.processDailyDigests(context.Background())

	if len(n.rawMessages) != 0 {
		t.Error("should not send digest outside time window")
	}
}

func TestProcessDailyDigests_SkipsAlreadySentToday(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifier{}
	dds := newMockDailyDigestStore()

	now := time.Now().In(time.UTC)
	digestTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	dds.digests[100] = struct{ enabled bool; digestTime string; lastSent time.Time }{
		true, digestTime, now,
	}

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.processDailyDigests(context.Background())

	if len(n.rawMessages) != 0 {
		t.Error("should not send digest if already sent today")
	}
}

// --- sendDailyDigest tests ---

func TestSendDailyDigest_Success(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifier{}
	dds := newMockDailyDigestStore()
	dds.stats[100] = []storage.DailySearchStats{
		{SearchName: "mazda", NewCount: 3, AvgPrice: 90000, BestPrice: 80000},
		{SearchName: "toyota", NewCount: 1, AvgPrice: 70000, BestPrice: 65000},
	}

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.sendDailyDigest(context.Background(), 100)

	if len(n.rawMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(n.rawMessages))
	}
	if n.rawMessages[0].recipient != "100" {
		t.Errorf("expected recipient '100', got %q", n.rawMessages[0].recipient)
	}
	if !dds.updated[100] {
		t.Error("expected last sent to be updated")
	}
}

func TestSendDailyDigest_EmptyStats(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifier{}
	dds := newMockDailyDigestStore()

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.sendDailyDigest(context.Background(), 100)

	if len(n.rawMessages) != 0 {
		t.Error("should not send for empty stats")
	}
}

func TestSendDailyDigest_StatsError(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifier{}
	dds := newMockDailyDigestStore()
	dds.statsErr = errors.New("db error")

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.sendDailyDigest(context.Background(), 100)

	if len(n.rawMessages) != 0 {
		t.Error("should not send on stats error")
	}
}

func TestSendDailyDigest_NotifyFails(t *testing.T) {
	cfg := testConfig()
	n := &mockNotifierWithRawErr{err: errors.New("notify failed")}
	dds := newMockDailyDigestStore()
	dds.stats[100] = []storage.DailySearchStats{
		{SearchName: "test", NewCount: 1, AvgPrice: 100000, BestPrice: 90000},
	}

	s, _ := NewWithOptions(cfg, nil, nil, n, testLogger(), Options{DailyDigestStore: dds})
	s.sendDailyDigest(context.Background(), 100)

	if dds.updated[100] {
		t.Error("should not update last sent when notify fails")
	}
}

type mockNotifierWithRawErr struct {
	mockNotifier
	err error
}

func (m *mockNotifierWithRawErr) NotifyRaw(_ context.Context, _ string, _ string) error {
	return m.err
}

// --- isUserPremium tests ---

func TestIsUserPremium_AlwaysTrue(t *testing.T) {
	cfg := testConfig()
	s, _ := New(cfg, nil, nil, nil, testLogger(), nil)

	if !s.isUserPremium(context.Background(), 100) {
		t.Error("expected true for all users (premium gating disabled)")
	}
}

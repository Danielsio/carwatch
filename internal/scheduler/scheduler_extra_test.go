package scheduler

import (
	"context"
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
	s, err := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{Health: h})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	if s.health != h {
		t.Error("health status not set")
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

package health

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// UserCounter counts active users (optional dependency).
type UserCounter interface {
	CountUsers(ctx context.Context) (int64, error)
}

// SearchCounter counts active searches (optional dependency).
type SearchCounter interface {
	CountAllSearches(ctx context.Context) (int64, error)
}

type SourceMetrics struct {
	FetchCount     atomic.Int64
	SuccessCount   atomic.Int64
	ErrorCount     atomic.Int64
	ChallengeCount atomic.Int64
	TotalMs        atomic.Int64
	LastSuccess    atomic.Int64
	LastError      atomic.Int64
}

type Status struct {
	startTime         time.Time
	cycleCount        atomic.Int64
	errorCount        atomic.Int64
	lastSuccessUnixNs atomic.Int64
	listingsFound     atomic.Int64
	notificationsSent atomic.Int64

	sourceMu sync.RWMutex
	sources  map[string]*SourceMetrics

	mu       sync.RWMutex
	users    UserCounter
	searches SearchCounter
}

func New() *Status {
	return &Status{
		startTime: time.Now(),
		sources:   make(map[string]*SourceMetrics),
	}
}

func (s *Status) SetUserCounter(u UserCounter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = u
}

func (s *Status) SetSearchCounter(sc SearchCounter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.searches = sc
}

func (s *Status) RecordSuccess() {
	s.lastSuccessUnixNs.Store(time.Now().UnixNano())
	s.cycleCount.Add(1)
}

func (s *Status) RecordError() {
	s.errorCount.Add(1)
	s.cycleCount.Add(1)
}

func (s *Status) RecordListingsFound(n int) {
	s.listingsFound.Add(int64(n))
}

func (s *Status) RecordNotificationSent() {
	s.notificationsSent.Add(1)
}

func (s *Status) RecordFetch(source string, duration time.Duration, err error) {
	m := s.getSource(source)
	m.FetchCount.Add(1)
	m.TotalMs.Add(duration.Milliseconds())
	if err == nil {
		m.SuccessCount.Add(1)
		m.LastSuccess.Store(time.Now().UnixNano())
	} else {
		m.ErrorCount.Add(1)
		m.LastError.Store(time.Now().UnixNano())
		if strings.Contains(err.Error(), "challenge") {
			m.ChallengeCount.Add(1)
		}
	}
}

func (s *Status) getSource(source string) *SourceMetrics {
	s.sourceMu.RLock()
	m, ok := s.sources[source]
	s.sourceMu.RUnlock()
	if ok {
		return m
	}

	s.sourceMu.Lock()
	defer s.sourceMu.Unlock()
	if s.sources == nil {
		s.sources = make(map[string]*SourceMetrics)
	}
	if m, ok = s.sources[source]; ok {
		return m
	}
	m = &SourceMetrics{}
	s.sources[source] = m
	return m
}

func (s *Status) Snapshot() map[string]any {
	cycles := s.cycleCount.Load()
	lastSuccessNs := s.lastSuccessUnixNs.Load()
	lastSuccess := time.Time{}
	if lastSuccessNs > 0 {
		lastSuccess = time.Unix(0, lastSuccessNs)
	}

	status := "ok"
	uptime := time.Since(s.startTime)
	if cycles > 0 && (lastSuccessNs == 0 || time.Since(lastSuccess) > 2*time.Hour) {
		if uptime > 5*time.Minute {
			status = "degraded"
		} else {
			status = "starting"
		}
	}

	resp := map[string]any{
		"status":             status,
		"uptime":             time.Since(s.startTime).String(),
		"cycles":             cycles,
		"errors":             s.errorCount.Load(),
		"last_success":       lastSuccess,
		"listings_found":     s.listingsFound.Load(),
		"notifications_sent": s.notificationsSent.Load(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.mu.RLock()
	users, searches := s.users, s.searches
	s.mu.RUnlock()

	if users != nil {
		if n, err := users.CountUsers(ctx); err == nil {
			resp["active_users"] = n
		}
	}
	if searches != nil {
		if n, err := searches.CountAllSearches(ctx); err == nil {
			resp["active_searches"] = n
		}
	}

	s.sourceMu.RLock()
	if len(s.sources) > 0 {
		srcMap := make(map[string]any, len(s.sources))
		for name, m := range s.sources {
			fetches := m.FetchCount.Load()
			successes := m.SuccessCount.Load()
			var avgMs int64
			if fetches > 0 {
				avgMs = m.TotalMs.Load() / fetches
			}
			var successRate float64
			if fetches > 0 {
				successRate = float64(successes) / float64(fetches)
			}
			entry := map[string]any{
				"fetches":        fetches,
				"successes":      successes,
				"errors":         m.ErrorCount.Load(),
				"challenges":     m.ChallengeCount.Load(),
				"avg_latency_ms": avgMs,
				"success_rate":   successRate,
			}
			if ns := m.LastSuccess.Load(); ns > 0 {
				entry["last_success"] = time.Unix(0, ns)
			}
			if ns := m.LastError.Load(); ns > 0 {
				entry["last_error"] = time.Unix(0, ns)
			}
			srcMap[name] = entry
		}
		resp["sources"] = srcMap
	}
	s.sourceMu.RUnlock()

	return resp
}

func (s *Status) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")

		resp := s.Snapshot()

		httpCode := http.StatusOK
		if resp["status"] == "degraded" {
			httpCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpCode)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

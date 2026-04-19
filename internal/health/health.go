package health

import (
	"context"
	"encoding/json"
	"net/http"
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

type Status struct {
	mu              sync.RWMutex
	lastSuccessTime time.Time
	cycleCount      int64
	errorCount      int64
	startTime       time.Time

	listingsFound     int64 // accessed via atomic
	notificationsSent int64 // accessed via atomic

	users    UserCounter
	searches SearchCounter
}

func New() *Status {
	return &Status{startTime: time.Now()}
}

// SetUserCounter attaches an optional UserCounter for active_users reporting.
func (s *Status) SetUserCounter(u UserCounter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = u
}

// SetSearchCounter attaches an optional SearchCounter for active_searches reporting.
func (s *Status) SetSearchCounter(sc SearchCounter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.searches = sc
}

func (s *Status) RecordSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSuccessTime = time.Now()
	s.cycleCount++
}

func (s *Status) RecordError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorCount++
	s.cycleCount++
}

// RecordListingsFound adds n to the total new listings discovered.
func (s *Status) RecordListingsFound(n int) {
	atomic.AddInt64(&s.listingsFound, int64(n))
}

// RecordNotificationSent increments the total notifications delivered.
func (s *Status) RecordNotificationSent() {
	atomic.AddInt64(&s.notificationsSent, 1)
}

// Snapshot returns the current health metrics as a map, suitable for JSON
// serialisation or embedding in other responses (e.g. /stats).
func (s *Status) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := "ok"
	if s.cycleCount > 0 && time.Since(s.lastSuccessTime) > 30*time.Minute {
		status = "degraded"
	}

	resp := map[string]any{
		"status":             status,
		"uptime":             time.Since(s.startTime).String(),
		"cycles":             s.cycleCount,
		"errors":             s.errorCount,
		"last_success":       s.lastSuccessTime,
		"listings_found":     atomic.LoadInt64(&s.listingsFound),
		"notifications_sent": atomic.LoadInt64(&s.notificationsSent),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if s.users != nil {
		if n, err := s.users.CountUsers(ctx); err == nil {
			resp["active_users"] = n
		}
	}
	if s.searches != nil {
		if n, err := s.searches.CountAllSearches(ctx); err == nil {
			resp["active_searches"] = n
		}
	}

	return resp
}

func (s *Status) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

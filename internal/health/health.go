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
	startTime         time.Time
	cycleCount        atomic.Int64
	errorCount        atomic.Int64
	lastSuccessUnixNs atomic.Int64
	listingsFound     atomic.Int64
	notificationsSent atomic.Int64

	mu       sync.RWMutex
	users    UserCounter
	searches SearchCounter
}

func New() *Status {
	return &Status{startTime: time.Now()}
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

func (s *Status) Snapshot() map[string]any {
	cycles := s.cycleCount.Load()
	lastSuccessNs := s.lastSuccessUnixNs.Load()
	lastSuccess := time.Time{}
	if lastSuccessNs > 0 {
		lastSuccess = time.Unix(0, lastSuccessNs)
	}

	status := "ok"
	if cycles > 0 && (lastSuccessNs == 0 || time.Since(lastSuccess) > 30*time.Minute) {
		status = "degraded"
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

package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Status struct {
	mu              sync.RWMutex
	lastSuccessTime time.Time
	cycleCount      int64
	errorCount      int64
	startTime       time.Time
}

func New() *Status {
	return &Status{startTime: time.Now()}
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

func (s *Status) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		status := "ok"
		httpCode := http.StatusOK

		if s.cycleCount > 0 && time.Since(s.lastSuccessTime) > 30*time.Minute {
			status = "degraded"
			httpCode = http.StatusServiceUnavailable
		}

		resp := map[string]any{
			"status":       status,
			"uptime":       time.Since(s.startTime).String(),
			"cycles":       s.cycleCount,
			"errors":       s.errorCount,
			"last_success": s.lastSuccessTime,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpCode)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

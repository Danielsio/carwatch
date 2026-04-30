package api

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu    sync.Mutex
	users map[int64]*bucket
	burst int
	every time.Duration
}

type bucket struct {
	tokens   int
	lastTick time.Time
	lastUsed time.Time
}

func newRateLimiter(burst int, every time.Duration) *rateLimiter {
	rl := &rateLimiter{
		users: make(map[int64]*bucket),
		burst: burst,
		every: every,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(chatID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.users[chatID]
	if !ok {
		b = &bucket{tokens: rl.burst, lastTick: time.Now()}
		rl.users[chatID] = b
	}

	now := time.Now()
	b.lastUsed = now

	elapsed := now.Sub(b.lastTick)
	refill := int(elapsed / rl.every)
	if refill > 0 {
		b.tokens = min(b.tokens+refill, rl.burst)
		b.lastTick = b.lastTick.Add(time.Duration(refill) * rl.every)
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for id, b := range rl.users {
			if b.lastUsed.Before(cutoff) {
				delete(rl.users, id)
			}
		}
		rl.mu.Unlock()
	}
}

func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	rl := newRateLimiter(60, time.Second)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chatID := chatIDFromContext(r.Context())
		if chatID != 0 && !rl.allow(chatID) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

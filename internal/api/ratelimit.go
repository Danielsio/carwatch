package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateLimiter struct {
	mu     sync.Mutex
	users  map[int64]*bucket
	burst  int
	every  time.Duration
	done   chan struct{}
	cancel context.CancelFunc
}

type bucket struct {
	tokens   int
	lastTick time.Time
	lastUsed time.Time
}

func newRateLimiter(burst int, every time.Duration) *rateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &rateLimiter{
		users:  make(map[int64]*bucket),
		burst:  burst,
		every:  every,
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go rl.cleanup(ctx)
	return rl
}

func (rl *rateLimiter) stop() {
	rl.cancel()
	<-rl.done
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

func (s *Server) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chatID, ok := chatIDFromContext(r.Context())
		if ok && !s.rl.allow(chatID) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ipBucket struct {
	tokens   int
	lastTick time.Time
	lastUsed time.Time
}

type ipRateLimiter struct {
	mu     sync.Mutex
	ips    map[string]*ipBucket
	burst  int
	every  time.Duration
	done   chan struct{}
	cancel context.CancelFunc
}

func newIPRateLimiter(burst int, every time.Duration) *ipRateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &ipRateLimiter{
		ips:    make(map[string]*ipBucket),
		burst:  burst,
		every:  every,
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go rl.cleanup(ctx)
	return rl
}

func (rl *ipRateLimiter) stop() {
	rl.cancel()
	<-rl.done
}

func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.ips[ip]
	if !ok {
		b = &ipBucket{tokens: rl.burst, lastTick: time.Now()}
		rl.ips[ip] = b
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

func (rl *ipRateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(rl.done)
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for ip, b := range rl.ips {
				if b.lastUsed.Before(cutoff) {
					delete(rl.ips, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.IndexByte(xff, ','); comma > 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) withIPRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !s.ipRL.allow(ip) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *rateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(rl.done)
			return
		case <-ticker.C:
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
}

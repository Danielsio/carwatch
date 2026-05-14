package api

import (
	"context"
	"log/slog"
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

const maxIPBuckets = 10_000

type ipRateLimiter struct {
	mu         sync.Mutex
	ips        map[string]*ipBucket
	burst      int
	every      time.Duration
	trustProxy bool
	done       chan struct{}
	cancel     context.CancelFunc
}

func newIPRateLimiter(burst int, every time.Duration, trustProxy bool) *ipRateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &ipRateLimiter{
		ips:        make(map[string]*ipBucket),
		burst:      burst,
		every:      every,
		trustProxy: trustProxy,
		done:       make(chan struct{}),
		cancel:     cancel,
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

	if len(rl.ips) >= maxIPBuckets {
		_, exists := rl.ips[ip]
		if !exists {
			slog.Warn("IP rate limiter full, rejecting unknown IP", "ip", ip)
			return false
		}
	}

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
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(rl.done)
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-5 * time.Minute)
			for ip, b := range rl.ips {
				if b.lastUsed.Before(cutoff) {
					delete(rl.ips, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// privateNets defines CIDRs that are considered trusted proxy sources.
var privateNets = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "::1/128",
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, _ := net.ParseCIDR(cidr)
		nets = append(nets, n)
	}
	return nets
}()

func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range privateNets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

func extractIP(r *http.Request, trustProxy bool) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !trustProxy || !isPrivateIP(host) {
		return host
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	return host
}

func (s *Server) withIPRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r, s.ipRL.trustProxy)
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

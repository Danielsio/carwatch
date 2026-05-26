package enricher

import (
	"context"
	"sync"
	"time"
)

// AdaptiveRateLimiter implements a token-bucket rate limiter with
// challenge-aware exponential backoff. It starts at baseDelay between
// requests and backs off on challenges, entering a cooldown period
// when the success rate drops too low.
type AdaptiveRateLimiter struct {
	mu              sync.Mutex
	baseDelay       time.Duration
	maxDelay        time.Duration
	cooldownDur     time.Duration
	currentDelay    time.Duration
	cooldownUntil   time.Time
	windowStart     time.Time
	windowSuccesses int
	windowTotal     int
	windowDuration  time.Duration
}

// NewAdaptiveRateLimiter creates a rate limiter with the given parameters.
func NewAdaptiveRateLimiter(baseDelay, maxDelay, cooldownDuration time.Duration) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		baseDelay:      baseDelay,
		maxDelay:       maxDelay,
		cooldownDur:    cooldownDuration,
		currentDelay:   baseDelay,
		windowStart:    time.Now(),
		windowDuration: 10 * time.Minute,
	}
}

// Wait blocks until the rate limiter allows the next request.
// Returns false if the context is cancelled during the wait.
func (r *AdaptiveRateLimiter) Wait(ctx context.Context) bool {
	r.mu.Lock()
	delay := r.currentDelay
	if now := time.Now(); now.Before(r.cooldownUntil) {
		delay = time.Until(r.cooldownUntil)
	}
	r.mu.Unlock()

	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// RecordSuccess records a successful request and reduces the delay.
func (r *AdaptiveRateLimiter) RecordSuccess() {
	r.mu.Lock()
	defer r.mu.Unlock()

	windowReset := r.resetWindowIfExpired()
	r.windowTotal++
	r.windowSuccesses++

	if !windowReset && r.currentDelay > r.baseDelay {
		r.currentDelay = r.currentDelay / 2
		if r.currentDelay < r.baseDelay {
			r.currentDelay = r.baseDelay
		}
	}
}

// RecordChallenge records a bot challenge and increases the delay.
// If the success rate over the sliding window drops below 50%,
// enters a cooldown period (preserving the current backoff level).
func (r *AdaptiveRateLimiter) RecordChallenge() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.resetWindowIfExpired()
	r.windowTotal++

	r.currentDelay = r.currentDelay * 2
	if r.currentDelay > r.maxDelay {
		r.currentDelay = r.maxDelay
	}

	if r.windowTotal >= 3 && r.successRate() < 0.5 {
		r.cooldownUntil = time.Now().Add(r.cooldownDur)
	}
}

// InCooldown reports whether the rate limiter is in a cooldown period.
func (r *AdaptiveRateLimiter) InCooldown() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return time.Now().Before(r.cooldownUntil)
}

// CurrentDelay returns the current delay between requests.
func (r *AdaptiveRateLimiter) CurrentDelay() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentDelay
}

func (r *AdaptiveRateLimiter) successRate() float64 {
	if r.windowTotal == 0 {
		return 1.0
	}
	return float64(r.windowSuccesses) / float64(r.windowTotal)
}

func (r *AdaptiveRateLimiter) resetWindowIfExpired() bool {
	if time.Since(r.windowStart) > r.windowDuration {
		r.windowStart = time.Now()
		r.windowSuccesses = 0
		r.windowTotal = 0
		return true
	}
	return false
}

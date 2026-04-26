package fetcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

type CircuitState int

const (
	StateClosed   CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var ErrCircuitOpen = fmt.Errorf("circuit breaker is open")

type CircuitBreaker struct {
	inner            Fetcher
	mu               sync.Mutex
	state            CircuitState
	failures         int
	failureThreshold int
	cooldown         time.Duration
	openedAt         time.Time
	probing          bool
}

func NewCircuitBreaker(inner Fetcher, threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		inner:            inner,
		state:            StateClosed,
		failureThreshold: threshold,
		cooldown:         cooldown,
	}
}

func (cb *CircuitBreaker) Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error) {
	cb.mu.Lock()
	switch cb.state {
	case StateOpen:
		if time.Since(cb.openedAt) >= cb.cooldown {
			cb.state = StateHalfOpen
		} else {
			cb.mu.Unlock()
			return nil, ErrCircuitOpen
		}
	case StateHalfOpen:
		if cb.probing {
			cb.mu.Unlock()
			return nil, ErrCircuitOpen
		}
	}
	cb.probing = cb.state == StateHalfOpen
	cb.mu.Unlock()

	defer func() {
		cb.mu.Lock()
		cb.probing = false
		cb.mu.Unlock()
	}()

	listings, err := cb.inner.Fetch(ctx, params)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		if cb.failures >= cb.failureThreshold || cb.state == StateHalfOpen {
			cb.state = StateOpen
			cb.openedAt = time.Now()
		}
		return nil, err
	}

	cb.failures = 0
	cb.state = StateClosed
	return listings, nil
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

package fetcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

type CircuitState int

const (
	StateClosed CircuitState = iota
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
	logger           *slog.Logger
}

func NewCircuitBreaker(inner Fetcher, threshold int, cooldown time.Duration, opts ...func(*CircuitBreaker)) *CircuitBreaker {
	cb := &CircuitBreaker{
		inner:            inner,
		state:            StateClosed,
		failureThreshold: threshold,
		cooldown:         cooldown,
		logger:           slog.Default(),
	}
	for _, o := range opts {
		o(cb)
	}
	return cb
}

func WithCBLogger(l *slog.Logger) func(*CircuitBreaker) {
	return func(cb *CircuitBreaker) {
		if l != nil {
			cb.logger = l
		}
	}
}

func (cb *CircuitBreaker) Fetch(ctx context.Context, params model.SourceParams) ([]model.RawListing, error) {
	cb.mu.Lock()
	switch cb.state {
	case StateOpen:
		if time.Since(cb.openedAt) >= cb.cooldown {
			cb.logger.Info("circuit breaker cooldown expired, transitioning to half-open",
				"cooldown", cb.cooldown.String(),
				"was_open_for", time.Since(cb.openedAt).Round(time.Second).String(),
			)
			cb.state = StateHalfOpen
		} else {
			remaining := cb.cooldown - time.Since(cb.openedAt)
			cb.mu.Unlock()
			return nil, fmt.Errorf("%w (resets in %s)", ErrCircuitOpen, remaining.Round(time.Second))
		}
	case StateHalfOpen:
		if cb.probing {
			cb.mu.Unlock()
			return nil, ErrCircuitOpen
		}
	}
	cb.probing = cb.state == StateHalfOpen
	cb.mu.Unlock()

	listings, err := cb.inner.Fetch(ctx, params)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.probing = false

	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			// Partial results with data indicate degradation but not total failure;
			// don't count them toward opening the circuit.
			if errors.Is(err, ErrPartialResults) && len(listings) > 0 {
				return listings, err
			}
			cb.failures++
			if cb.failures >= cb.failureThreshold || cb.state == StateHalfOpen {
				prev := cb.state
				cb.state = StateOpen
				cb.openedAt = time.Now()
				cb.logger.Warn("circuit breaker opened",
					"previous_state", prev.String(),
					"failures", cb.failures,
					"threshold", cb.failureThreshold,
					"cooldown", cb.cooldown.String(),
					"last_error", err.Error(),
				)
			}
		}
		return nil, err
	}

	if cb.state != StateClosed {
		cb.logger.Info("circuit breaker recovered, closing",
			"previous_state", cb.state.String(),
			"failures_before_reset", cb.failures,
		)
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

package fetcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

type mockFetcherCB struct {
	listings []model.RawListing
	err      error
	calls    int
}

func (m *mockFetcherCB) Fetch(_ context.Context, _ model.SourceParams) ([]model.RawListing, error) {
	m.calls++
	return m.listings, m.err
}

func TestCircuitBreaker_ClosedState_PassesThrough(t *testing.T) {
	inner := &mockFetcherCB{listings: []model.RawListing{{Token: "a"}}}
	cb := NewCircuitBreaker(inner, 3, 30*time.Second)

	listings, err := cb.Fetch(context.Background(), model.SourceParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing, got %d", len(listings))
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want closed", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	inner := &mockFetcherCB{err: errors.New("timeout")}
	cb := NewCircuitBreaker(inner, 3, 30*time.Second)

	for i := 0; i < 3; i++ {
		_, _ = cb.Fetch(context.Background(), model.SourceParams{})
	}

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want open after %d failures", cb.State(), cb.Failures())
	}
}

func TestCircuitBreaker_OpenState_RejectsCalls(t *testing.T) {
	inner := &mockFetcherCB{err: errors.New("timeout")}
	cb := NewCircuitBreaker(inner, 3, 30*time.Second)

	for i := 0; i < 3; i++ {
		_, _ = cb.Fetch(context.Background(), model.SourceParams{})
	}
	inner.calls = 0

	_, err := cb.Fetch(context.Background(), model.SourceParams{})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
	if inner.calls != 0 {
		t.Errorf("inner should not be called when circuit is open, got %d calls", inner.calls)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	inner := &mockFetcherCB{err: errors.New("timeout")}
	cb := NewCircuitBreaker(inner, 3, 10*time.Millisecond)

	for i := 0; i < 3; i++ {
		_, _ = cb.Fetch(context.Background(), model.SourceParams{})
	}

	time.Sleep(15 * time.Millisecond)

	inner.err = nil
	inner.listings = []model.RawListing{{Token: "recovered"}}

	listings, err := cb.Fetch(context.Background(), model.SourceParams{})
	if err != nil {
		t.Fatalf("half-open probe should succeed: %v", err)
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing from probe, got %d", len(listings))
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want closed after successful probe", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure_ReOpens(t *testing.T) {
	inner := &mockFetcherCB{err: errors.New("timeout")}
	cb := NewCircuitBreaker(inner, 3, 10*time.Millisecond)

	for i := 0; i < 3; i++ {
		_, _ = cb.Fetch(context.Background(), model.SourceParams{})
	}

	time.Sleep(15 * time.Millisecond)

	_, err := cb.Fetch(context.Background(), model.SourceParams{})
	if err == nil {
		t.Fatal("expected error from failing probe")
	}
	if cb.State() != StateOpen {
		t.Errorf("state = %v, want open after failed probe", cb.State())
	}
}

func TestCircuitBreaker_ResetsCounterOnSuccess(t *testing.T) {
	inner := &mockFetcherCB{err: errors.New("flaky")}
	cb := NewCircuitBreaker(inner, 3, 30*time.Second)

	_, _ = cb.Fetch(context.Background(), model.SourceParams{})
	_, _ = cb.Fetch(context.Background(), model.SourceParams{})

	inner.err = nil
	inner.listings = []model.RawListing{{Token: "ok"}}
	_, _ = cb.Fetch(context.Background(), model.SourceParams{})

	if cb.Failures() != 0 {
		t.Errorf("failures = %d, want 0 after success", cb.Failures())
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want closed", cb.State())
	}
}

func TestCircuitBreaker_DoesNotOpenBelowThreshold(t *testing.T) {
	inner := &mockFetcherCB{err: errors.New("error")}
	cb := NewCircuitBreaker(inner, 5, 30*time.Second)

	for i := 0; i < 4; i++ {
		_, _ = cb.Fetch(context.Background(), model.SourceParams{})
	}

	if cb.State() != StateClosed {
		t.Errorf("state = %v, want closed (only %d failures, threshold 5)", cb.State(), cb.Failures())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestCircuitBreaker_ChallengeError_CountsAsFailure(t *testing.T) {
	inner := &mockFetcherCB{err: ErrChallenge}
	cb := NewCircuitBreaker(inner, 2, 30*time.Second)

	_, _ = cb.Fetch(context.Background(), model.SourceParams{})
	_, _ = cb.Fetch(context.Background(), model.SourceParams{})

	if cb.State() != StateOpen {
		t.Errorf("challenge errors should count toward circuit opening, state = %v", cb.State())
	}
}

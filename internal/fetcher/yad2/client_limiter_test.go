package yad2

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestInFlightLimiter_RefusesAboveCap verifies that once the limiter is
// saturated with slow (still-running) fetches, further calls fail fast with
// errTooManyInFlight rather than spawning unbounded goroutines.
func TestInFlightLimiter_RefusesAboveCap(t *testing.T) {
	const cap = 4
	l := newInFlightLimiter(cap)

	release := make(chan struct{})
	slow := func() (*HTTPResult, error) {
		<-release // block until the test lets it finish
		return &HTTPResult{StatusCode: 200}, nil
	}

	// Saturate every slot with a goroutine whose context is already cancelled,
	// so each call returns immediately but leaves its fetch goroutine running
	// (and thus holding its slot) until we close(release).
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	var wg sync.WaitGroup
	for i := 0; i < cap; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := l.run(cancelled, slow); !errors.Is(err, context.Canceled) {
				t.Errorf("saturating call: expected context.Canceled, got %v", err)
			}
		}()
	}
	wg.Wait()

	// All slots are now held by the still-blocked slow fetches. A fresh call
	// must be refused immediately even with a healthy context.
	if _, err := l.run(context.Background(), slow); !errors.Is(err, errTooManyInFlight) {
		t.Fatalf("expected errTooManyInFlight when saturated, got %v", err)
	}

	// Let the blocked fetches finish, freeing their slots.
	close(release)

	// The limiter must recover: a new call now succeeds.
	if !waitForFreeSlot(t, l) {
		t.Fatal("limiter did not recover capacity after in-flight fetches completed")
	}
}

// TestInFlightLimiter_PassesThroughResult verifies the happy path returns the
// fetch result unchanged and holds no slot afterward.
func TestInFlightLimiter_PassesThroughResult(t *testing.T) {
	l := newInFlightLimiter(2)
	want := &HTTPResult{StatusCode: 204, Body: []byte("ok")}
	got, err := l.run(context.Background(), func() (*HTTPResult, error) {
		return want, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("result not passed through: got %v", got)
	}
	// Both slots should be free.
	for i := 0; i < cap(l.sem); i++ {
		select {
		case l.sem <- struct{}{}:
		default:
			t.Fatalf("slot %d not released after successful run", i)
		}
	}
}

// waitForFreeSlot retries a trivial run until a slot frees up (the orphaned
// fetch goroutines release asynchronously) or a deadline passes.
func waitForFreeSlot(t *testing.T, l *inFlightLimiter) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, err := l.run(context.Background(), func() (*HTTPResult, error) {
			return &HTTPResult{StatusCode: 200}, nil
		})
		if err == nil {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

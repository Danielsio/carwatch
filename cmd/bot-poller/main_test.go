package main

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/health"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestPollingLoop_BackoffGrowsOnImmediateFailures(t *testing.T) {
	h := health.New()
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	poll := func(_ context.Context) {
		n := calls.Add(1)
		if n >= 2 {
			cancel()
		}
	}

	bo := backoffConfig{initial: 100 * time.Millisecond, max: 5 * time.Second, resetThreshold: 30 * time.Second}
	start := time.Now()
	pollingLoopWithConfig(ctx, h, poll, testLogger(), bo)
	elapsed := time.Since(start)

	if calls.Load() < 2 {
		t.Errorf("expected at least 2 poll calls, got %d", calls.Load())
	}
	if elapsed < 90*time.Millisecond {
		t.Errorf("expected at least ~100ms backoff, got %v", elapsed)
	}
}

func TestPollingLoop_BackoffResetsAfterLongRunningPoll(t *testing.T) {
	h := health.New()

	var mu sync.Mutex
	var delays []time.Duration
	lastPollEnd := time.Now()

	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	bo := backoffConfig{
		initial:        100 * time.Millisecond,
		max:            5 * time.Second,
		resetThreshold: 200 * time.Millisecond,
	}

	poll := func(_ context.Context) {
		n := calls.Add(1)
		mu.Lock()
		now := time.Now()
		if n > 1 {
			delays = append(delays, now.Sub(lastPollEnd))
		}
		mu.Unlock()

		switch n {
		case 1:
			// Immediate failure — backoff grows from initial (100ms)
		case 2:
			// Run longer than resetThreshold to trigger reset
			time.Sleep(250 * time.Millisecond)
		case 3:
			// After reset, backoff should be back to initial
			cancel()
		}

		mu.Lock()
		lastPollEnd = time.Now()
		mu.Unlock()
	}

	pollingLoopWithConfig(ctx, h, poll, testLogger(), bo)

	if calls.Load() < 3 {
		t.Fatalf("expected 3 poll calls, got %d", calls.Load())
	}

	mu.Lock()
	defer mu.Unlock()
	// delays[0] = gap between call 1 end and call 2 start = initial backoff (100ms)
	// delays[1] = gap between call 2 end and call 3 start = reset backoff (100ms, not 200ms)
	if len(delays) < 2 {
		t.Fatalf("expected 2 delay measurements, got %d", len(delays))
	}
	// After call 2 ran >resetThreshold, backoff should reset to initial (100ms).
	// Without reset, it would be 200ms (100ms * 2).
	if delays[1] > 180*time.Millisecond {
		t.Errorf("backoff after long run should be ~100ms (reset), got %v", delays[1])
	}
}

func TestPollingLoop_ShutdownOnContextCancel(t *testing.T) {
	h := health.New()
	ctx, cancel := context.WithCancel(context.Background())

	poll := func(_ context.Context) {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		pollingLoop(ctx, h, poll, testLogger())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pollingLoop did not exit after context cancel")
	}
}

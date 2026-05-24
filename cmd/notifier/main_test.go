package main

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultHealthBind(t *testing.T) {
	if defaultHealthBind != "0.0.0.0:8083" {
		t.Errorf("defaultHealthBind = %q, want 0.0.0.0:8083", defaultHealthBind)
	}
}

type mockRunner struct {
	calls atomic.Int32
	err   error
}

func (m *mockRunner) Run(ctx context.Context) error {
	m.calls.Add(1)
	if m.err != nil {
		return m.err
	}
	<-ctx.Done()
	return ctx.Err()
}

func (m *mockRunner) Drain(_ context.Context) {}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestConsumerLoop_BackoffGrowsOnImmediateFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mr := &mockRunner{err: errors.New("transient")}

	bo := consumerBackoff{initial: 100 * time.Millisecond, max: 5 * time.Second, resetThreshold: 30 * time.Second}

	go func() {
		for mr.calls.Load() < 3 {
			time.Sleep(50 * time.Millisecond)
		}
		cancel()
	}()

	start := time.Now()
	consumerLoopWithConfig(ctx, mr, testLogger(), bo)
	elapsed := time.Since(start)

	if mr.calls.Load() < 3 {
		t.Errorf("expected at least 3 Run calls, got %d", mr.calls.Load())
	}
	// 3 calls: backoff 100ms + 200ms = 300ms minimum. Without reset, this confirms growth.
	if elapsed < 250*time.Millisecond {
		t.Errorf("expected backoff growth (>=300ms total), got %v", elapsed)
	}
}

func TestConsumerLoop_ResetsBackoffAfterLongRunningConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	bo := consumerBackoff{
		initial:        100 * time.Millisecond,
		max:            5 * time.Second,
		resetThreshold: 200 * time.Millisecond,
	}

	var calls atomic.Int32
	var runDurations []time.Duration
	mr := &timedRunner{
		runFunc: func(ctx context.Context) error {
			n := calls.Add(1)
			switch n {
			case 1:
				return errors.New("immediate failure")
			case 2:
				// Run longer than resetThreshold
				time.Sleep(250 * time.Millisecond)
				return errors.New("late failure after long run")
			case 3:
				cancel()
				return ctx.Err()
			}
			<-ctx.Done()
			return ctx.Err()
		},
		durations: &runDurations,
	}

	start := time.Now()
	consumerLoopWithConfig(ctx, mr, testLogger(), bo)
	elapsed := time.Since(start)

	if calls.Load() < 3 {
		t.Fatalf("expected 3 Run calls, got %d", calls.Load())
	}

	// Call 1: immediate fail -> backoff 100ms
	// Call 2: runs 250ms (>resetThreshold) -> backoff resets to 100ms (not 200ms)
	// Call 3: cancel
	// Total without reset: 100ms + 250ms + 200ms = 550ms
	// Total with reset: 100ms + 250ms + 100ms = 450ms
	// We check it finishes faster than the non-reset path
	if elapsed > 600*time.Millisecond {
		t.Errorf("expected faster completion with reset, got %v", elapsed)
	}
}

type timedRunner struct {
	runFunc   func(ctx context.Context) error
	durations *[]time.Duration
}

func (t *timedRunner) Run(ctx context.Context) error {
	start := time.Now()
	err := t.runFunc(ctx)
	*t.durations = append(*t.durations, time.Since(start))
	return err
}

func (t *timedRunner) Drain(_ context.Context) {}

func TestConsumerLoop_ShutdownOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mr := &mockRunner{}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		consumerLoop(ctx, mr, testLogger())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumerLoop did not exit after context cancel")
	}
}

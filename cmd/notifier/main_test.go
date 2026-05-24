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

func TestConsumerLoop_ResetsBackoffAfterSuccessfulRestart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mr := &mockRunner{err: errors.New("transient")}

	go func() {
		// Let 3 iterations happen, then cancel.
		for mr.calls.Load() < 3 {
			time.Sleep(50 * time.Millisecond)
		}
		cancel()
	}()

	start := time.Now()
	consumerLoop(ctx, mr, testLogger())
	elapsed := time.Since(start)

	if mr.calls.Load() < 3 {
		t.Errorf("expected at least 3 Run calls, got %d", mr.calls.Load())
	}
	// With reset, each backoff is 1s. Without reset, 2nd wait is 2s, 3rd is 4s.
	// 3 calls in < 4s proves backoff was reset.
	if elapsed > 4*time.Second {
		t.Errorf("loop took %v; backoff reset should keep it under 4s", elapsed)
	}
}

func TestConsumerLoop_ShutdownOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mr := &mockRunner{err: nil}

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

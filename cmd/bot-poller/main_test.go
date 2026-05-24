package main

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/health"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestPollingLoop_ResetsBackoffAfterSuccessfulRestart(t *testing.T) {
	h := health.New()
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	poll := func(_ context.Context) {
		n := calls.Add(1)
		if n >= 3 {
			cancel()
		}
	}

	start := time.Now()
	pollingLoop(ctx, h, poll, testLogger())
	elapsed := time.Since(start)

	if calls.Load() < 3 {
		t.Errorf("expected at least 3 poll calls, got %d", calls.Load())
	}
	// With backoff reset, each restart waits 1s. Without reset, the 2nd
	// restart would wait 2s. 3 calls in < 3s proves backoff was reset.
	if elapsed > 3*time.Second {
		t.Errorf("loop took %v; backoff reset should keep it under 3s", elapsed)
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

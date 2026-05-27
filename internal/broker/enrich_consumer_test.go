package broker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestEnrichConsumer_ProcessesPriorityOrder(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	// Publish in reverse priority order.
	for _, p := range []int{3, 1, 2} {
		req := EnrichRequest{
			Token:    fmt.Sprintf("tok-p%d", p),
			Priority: p,
			Source:   "test",
		}
		if err := pub.PublishEnrich(ctx, req); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	var mu sync.Mutex
	var processed []string

	fn := func(_ context.Context, req EnrichRequest) error {
		mu.Lock()
		processed = append(processed, req.Token)
		mu.Unlock()
		return nil
	}

	cons, err := NewEnrichConsumer(s.Addr(), "", 0, fn, testLogger())
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_ = cons.Run(runCtx)

	mu.Lock()
	defer mu.Unlock()

	if len(processed) != 3 {
		t.Fatalf("expected 3 processed, got %d", len(processed))
	}
	if processed[0] != "tok-p1" {
		t.Errorf("first processed = %q, want tok-p1 (highest priority)", processed[0])
	}
	if processed[1] != "tok-p2" {
		t.Errorf("second processed = %q, want tok-p2", processed[1])
	}
	if processed[2] != "tok-p3" {
		t.Errorf("third processed = %q, want tok-p3 (lowest priority)", processed[2])
	}
}

func TestEnrichConsumer_AcksOnSuccess(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req := EnrichRequest{Token: "ack-test", Priority: 1, Source: "test"}
	if err := pub.PublishEnrich(ctx, req); err != nil {
		t.Fatalf("publish: %v", err)
	}

	called := false
	fn := func(_ context.Context, r EnrichRequest) error {
		called = true
		if r.Token != "ack-test" {
			t.Errorf("token = %q, want ack-test", r.Token)
		}
		return nil
	}

	cons, err := NewEnrichConsumer(s.Addr(), "", 0, fn, testLogger())
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = cons.Run(runCtx)

	if !called {
		t.Error("enrich function was not called")
	}

	// Verify message was acknowledged (no pending).
	pending, err := client.XPending(ctx, EnrichStreamName, EnrichGroupName).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("expected 0 pending after ack, got %d", pending.Count)
	}
}

func TestEnrichConsumer_ConnectionFailure(t *testing.T) {
	fn := func(_ context.Context, _ EnrichRequest) error { return nil }
	_, err := NewEnrichConsumer("localhost:1", "", 0, fn, testLogger())
	if err == nil {
		t.Error("expected error for invalid Redis address")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

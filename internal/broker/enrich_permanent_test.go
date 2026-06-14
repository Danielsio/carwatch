package broker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestEnrichConsumer_PermanentErrorDeadLettersImmediately verifies that an
// EnrichFunc error wrapping ErrPermanent is moved to the dead-letter stream and
// acked on the first attempt, rather than left pending for retry (F6).
func TestEnrichConsumer_PermanentErrorDeadLettersImmediately(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()
	if err := pub.PublishEnrich(ctx, EnrichRequest{Token: "gone-token", Priority: 1, Source: "test"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	calls := 0
	fn := func(_ context.Context, _ EnrichRequest) error {
		calls++
		return fmt.Errorf("listing removed: %w", ErrPermanent)
	}

	cons, err := NewEnrichConsumer(s.Addr(), "", 0, fn, testLogger())
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = cons.Run(runCtx)

	if calls != 1 {
		t.Fatalf("permanent error should be attempted exactly once, got %d", calls)
	}

	// The original message must be acked (not pending).
	pending, err := client.XPending(ctx, EnrichStreamName, EnrichGroupName).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("expected 0 pending after permanent dead-letter, got %d", pending.Count)
	}

	// And it must have landed in the dead-letter stream.
	dlqLen, err := client.XLen(ctx, EnrichDeadLetterStream).Result()
	if err != nil {
		t.Fatalf("xlen dlq: %v", err)
	}
	if dlqLen != 1 {
		t.Errorf("expected 1 message in dead-letter stream, got %d", dlqLen)
	}
}

// TestEnrichConsumer_TransientErrorStaysPending verifies that a plain (non-
// permanent) error leaves the message pending for retry instead of
// dead-lettering.
func TestEnrichConsumer_TransientErrorStaysPending(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()
	if err := pub.PublishEnrich(ctx, EnrichRequest{Token: "flaky-token", Priority: 1, Source: "test"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	fn := func(_ context.Context, _ EnrichRequest) error {
		return fmt.Errorf("temporary network blip")
	}

	cons, err := NewEnrichConsumer(s.Addr(), "", 0, fn, testLogger())
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = cons.Run(runCtx)

	// Transient failure: message stays pending, nothing dead-lettered.
	pending, err := client.XPending(ctx, EnrichStreamName, EnrichGroupName).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 1 {
		t.Errorf("expected 1 pending after transient error, got %d", pending.Count)
	}
	dlqLen, _ := client.XLen(ctx, EnrichDeadLetterStream).Result()
	if dlqLen != 0 {
		t.Errorf("expected 0 dead-lettered for transient error, got %d", dlqLen)
	}
}

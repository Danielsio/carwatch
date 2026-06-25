package broker

import (
	"context"
	"encoding/json"
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

func TestEnrichConsumer_DeadLettersAfterMaxRetries(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req := EnrichRequest{Token: "stuck-token", Priority: 3, Source: "test"}
	if err := pub.PublishEnrich(ctx, req); err != nil {
		t.Fatalf("publish: %v", err)
	}

	fn := func(_ context.Context, _ EnrichRequest) error {
		return fmt.Errorf("anti-bot challenge detected")
	}

	cons, err := NewEnrichConsumer(s.Addr(), "", 0, fn, testLogger())
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()
	cons.reclaimIdleThreshold = 0

	// Initial read creates PEL entry (delivery count = 1).
	streams, err := cons.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    EnrichGroupName,
		Consumer: cons.consumer,
		Streams:  []string{EnrichStreamName, ">"},
		Count:    10,
		Block:    time.Second,
	}).Result()
	if err != nil {
		t.Fatalf("xreadgroup: %v", err)
	}
	cons.processBatch(ctx, streams[0].Messages)

	// Each reclaimPending call XClaims (incrementing delivery count) and reprocesses.
	// After enrichMaxRetries deliveries, the message should be dead-lettered.
	for i := 0; i < enrichMaxRetries+1; i++ {
		cons.reclaimPending(ctx)
	}

	// Verify message was dead-lettered.
	dlq, err := client.XRange(ctx, EnrichDeadLetterStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange dead-letter: %v", err)
	}
	if len(dlq) != 1 {
		t.Fatalf("expected 1 dead-lettered message, got %d", len(dlq))
	}

	var dlqReq EnrichRequest
	data, ok := dlq[0].Values["data"].(string)
	if !ok {
		t.Fatal("dead-lettered message missing data field")
	}
	if err := json.Unmarshal([]byte(data), &dlqReq); err != nil {
		t.Fatalf("unmarshal DLQ message: %v", err)
	}
	if dlqReq.Token != "stuck-token" {
		t.Errorf("DLQ token = %q, want stuck-token", dlqReq.Token)
	}

	// Verify no more pending messages.
	pending, err := client.XPending(ctx, EnrichStreamName, EnrichGroupName).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("expected 0 pending after dead-letter, got %d", pending.Count)
	}
}

func TestEnrichConsumer_ConnectionFailure(t *testing.T) {
	fn := func(_ context.Context, _ EnrichRequest) error { return nil }
	_, err := NewEnrichConsumer("localhost:1", "", 0, fn, testLogger())
	if err == nil {
		t.Error("expected error for invalid Redis address")
	}
}

func TestEnrichConsumer_AckRemovesPendingSet(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req := EnrichRequest{Token: "tok-ack-pending", Priority: 1, Source: "test"}
	published, err := pub.PublishEnrichDedup(ctx, req)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !published {
		t.Fatal("expected token to be published")
	}

	// Verify token is in pending set
	isMember, err := client.SIsMember(ctx, EnrichPendingSet, "tok-ack-pending").Result()
	if err != nil {
		t.Fatalf("sismember before: %v", err)
	}
	if !isMember {
		t.Fatal("token should be in pending set before processing")
	}

	fn := func(_ context.Context, r EnrichRequest) error {
		if r.Token != "tok-ack-pending" {
			t.Errorf("token = %q, want tok-ack-pending", r.Token)
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

	// Verify token was removed from pending set
	isMember, err = client.SIsMember(ctx, EnrichPendingSet, "tok-ack-pending").Result()
	if err != nil {
		t.Fatalf("sismember after: %v", err)
	}
	if isMember {
		t.Error("token should be removed from pending set after processing")
	}
}

func TestEnrichConsumer_DeadLetterRemovesPendingSet(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req := EnrichRequest{Token: "tok-dlq-pending", Priority: 3, Source: "test"}
	published, err := pub.PublishEnrichDedup(ctx, req)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !published {
		t.Fatal("expected token to be published")
	}

	// Verify token is in pending set
	isMember, err := client.SIsMember(ctx, EnrichPendingSet, "tok-dlq-pending").Result()
	if err != nil {
		t.Fatalf("sismember before: %v", err)
	}
	if !isMember {
		t.Fatal("token should be in pending set before dead-lettering")
	}

	fn := func(_ context.Context, _ EnrichRequest) error {
		return fmt.Errorf("persistent error")
	}

	cons, err := NewEnrichConsumer(s.Addr(), "", 0, fn, testLogger())
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()
	cons.reclaimIdleThreshold = 0

	// Initial read creates PEL entry
	streams, err := cons.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    EnrichGroupName,
		Consumer: cons.consumer,
		Streams:  []string{EnrichStreamName, ">"},
		Count:    10,
		Block:    time.Second,
	}).Result()
	if err != nil {
		t.Fatalf("xreadgroup: %v", err)
	}
	cons.processBatch(ctx, streams[0].Messages)

	// Reclaim until dead-lettered
	for i := 0; i < enrichMaxRetries+1; i++ {
		cons.reclaimPending(ctx)
	}

	// Verify token was removed from pending set
	isMember, err = client.SIsMember(ctx, EnrichPendingSet, "tok-dlq-pending").Result()
	if err != nil {
		t.Fatalf("sismember after: %v", err)
	}
	if isMember {
		t.Error("token should be removed from pending set after dead-lettering")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

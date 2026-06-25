package broker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestEnrichPublisher_PublishEnrich(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req := EnrichRequest{
		Token:      "tok-123",
		Priority:   1,
		SearchIDs:  []int64{10, 20},
		Source:     "match",
		EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := pub.PublishEnrich(ctx, req); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msgs, err := client.XRange(ctx, EnrichStreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	data, ok := msgs[0].Values["data"].(string)
	if !ok {
		t.Fatal("expected data field as string")
	}

	var decoded EnrichRequest
	if err := json.Unmarshal([]byte(data), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Token != "tok-123" {
		t.Errorf("token = %q, want tok-123", decoded.Token)
	}
	if decoded.Priority != 1 {
		t.Errorf("priority = %d, want 1", decoded.Priority)
	}
	if decoded.Source != "match" {
		t.Errorf("source = %q, want match", decoded.Source)
	}
}

func TestEnrichPublisher_QueueLen(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	for i := range 5 {
		req := EnrichRequest{Token: string(rune('a' + i)), Priority: 3, Source: "backfill"}
		if err := pub.PublishEnrich(ctx, req); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	length, err := pub.EnrichQueueLen(ctx)
	if err != nil {
		t.Fatalf("queue len: %v", err)
	}
	if length != 5 {
		t.Errorf("queue length = %d, want 5", length)
	}
}

func TestPublishEnrichDedup_FirstPublish(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req := EnrichRequest{
		Token:      "tok-dedup-1",
		Priority:   1,
		SearchIDs:  []int64{10},
		Source:     "scheduler",
		EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
	}

	published, err := pub.PublishEnrichDedup(ctx, req)
	if err != nil {
		t.Fatalf("publish dedup: %v", err)
	}
	if !published {
		t.Error("expected token to be published on first call")
	}

	// Verify message was added to stream
	msgs, err := client.XRange(ctx, EnrichStreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in stream, got %d", len(msgs))
	}

	// Verify token was added to pending set
	isMember, err := client.SIsMember(ctx, EnrichPendingSet, "tok-dedup-1").Result()
	if err != nil {
		t.Fatalf("sismember: %v", err)
	}
	if !isMember {
		t.Error("expected token to be in pending set")
	}
}

func TestPublishEnrichDedup_DuplicateSkipped(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req := EnrichRequest{
		Token:    "tok-dedup-2",
		Priority: 1,
		Source:   "scheduler",
	}

	// First publish should succeed
	published, err := pub.PublishEnrichDedup(ctx, req)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if !published {
		t.Error("expected first publish to succeed")
	}

	// Second publish should be skipped
	published2, err := pub.PublishEnrichDedup(ctx, req)
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if published2 {
		t.Error("expected second publish to be skipped due to deduplication")
	}

	// Verify only 1 message in stream
	msgs, err := client.XRange(ctx, EnrichStreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message in stream, got %d", len(msgs))
	}
}

func TestPublishEnrichDedup_DifferentTokensBothPublished(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	req1 := EnrichRequest{Token: "tok-a", Priority: 1, Source: "scheduler"}
	req2 := EnrichRequest{Token: "tok-b", Priority: 2, Source: "scheduler"}

	// Both should be published
	ok1, err := pub.PublishEnrichDedup(ctx, req1)
	if err != nil {
		t.Fatalf("publish req1: %v", err)
	}
	if !ok1 {
		t.Error("expected req1 to be published")
	}

	ok2, err := pub.PublishEnrichDedup(ctx, req2)
	if err != nil {
		t.Fatalf("publish req2: %v", err)
	}
	if !ok2 {
		t.Error("expected req2 to be published")
	}

	// Verify 2 messages in stream
	msgs, err := client.XRange(ctx, EnrichStreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages in stream, got %d", len(msgs))
	}

	// Verify both tokens in pending set
	count, err := client.SCard(ctx, EnrichPendingSet).Result()
	if err != nil {
		t.Fatalf("scard: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 tokens in pending set, got %d", count)
	}
}

func TestRemovePending(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	// Add token to pending set
	if err := client.SAdd(ctx, EnrichPendingSet, "tok-remove").Err(); err != nil {
		t.Fatalf("sadd: %v", err)
	}

	// Verify it's there
	isMember, err := client.SIsMember(ctx, EnrichPendingSet, "tok-remove").Result()
	if err != nil {
		t.Fatalf("sismember before: %v", err)
	}
	if !isMember {
		t.Fatal("token should be in pending set before removal")
	}

	// Remove it
	if err := pub.RemovePending(ctx, "tok-remove"); err != nil {
		t.Fatalf("remove pending: %v", err)
	}

	// Verify it's gone
	isMember, err = client.SIsMember(ctx, EnrichPendingSet, "tok-remove").Result()
	if err != nil {
		t.Fatalf("sismember after: %v", err)
	}
	if isMember {
		t.Error("token should be removed from pending set")
	}
}

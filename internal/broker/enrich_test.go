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

func TestPurgeEnrichedEntries_RemovesFullyEnriched(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	// Seed stream with 3 messages
	for i, token := range []string{"tok-1", "tok-2", "tok-3"} {
		req := EnrichRequest{
			Token:      token,
			Priority:   i + 1,
			Source:     "test",
			EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := pub.PublishEnrich(ctx, req); err != nil {
			t.Fatalf("publish %s: %v", token, err)
		}
		// Add to pending set
		if err := client.SAdd(ctx, EnrichPendingSet, token).Err(); err != nil {
			t.Fatalf("sadd %s: %v", token, err)
		}
	}

	// Checker says tok-1 and tok-3 are enriched
	checker := func(ctx context.Context, tokens []string) (map[string]bool, error) {
		result := make(map[string]bool)
		for _, t := range tokens {
			result[t] = t == "tok-1" || t == "tok-3"
		}
		return result, nil
	}

	purged, err := pub.PurgeEnrichedEntries(ctx, checker)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 2 {
		t.Errorf("purged = %d, want 2", purged)
	}

	// Verify stream has 1 message (tok-2)
	msgs, err := client.XRange(ctx, EnrichStreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in stream, got %d", len(msgs))
	}

	data, ok := msgs[0].Values["data"].(string)
	if !ok {
		t.Fatal("expected data field")
	}
	var req EnrichRequest
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Token != "tok-2" {
		t.Errorf("remaining token = %q, want tok-2", req.Token)
	}

	// Verify pending set has 1 token (tok-2)
	count, err := client.SCard(ctx, EnrichPendingSet).Result()
	if err != nil {
		t.Fatalf("scard: %v", err)
	}
	if count != 1 {
		t.Errorf("pending set count = %d, want 1", count)
	}
	isMember, err := client.SIsMember(ctx, EnrichPendingSet, "tok-2").Result()
	if err != nil {
		t.Fatalf("sismember: %v", err)
	}
	if !isMember {
		t.Error("tok-2 should be in pending set")
	}
}

func TestPurgeEnrichedEntries_EmptyStream(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	checker := func(ctx context.Context, tokens []string) (map[string]bool, error) {
		t.Error("checker should not be called for empty stream")
		return nil, nil
	}

	purged, err := pub.PurgeEnrichedEntries(ctx, checker)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged = %d, want 0", purged)
	}
}

func TestPurgeEnrichedEntries_NoneEnriched(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	// Seed stream with 3 messages
	for i, token := range []string{"tok-a", "tok-b", "tok-c"} {
		req := EnrichRequest{
			Token:      token,
			Priority:   i + 1,
			Source:     "test",
			EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := pub.PublishEnrich(ctx, req); err != nil {
			t.Fatalf("publish %s: %v", token, err)
		}
	}

	// Checker says none are enriched
	checker := func(ctx context.Context, tokens []string) (map[string]bool, error) {
		result := make(map[string]bool)
		for _, t := range tokens {
			result[t] = false
		}
		return result, nil
	}

	purged, err := pub.PurgeEnrichedEntries(ctx, checker)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged = %d, want 0", purged)
	}

	// Verify all 3 messages remain
	msgs, err := client.XRange(ctx, EnrichStreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages in stream, got %d", len(msgs))
	}
}

func TestTrimDeadLetterStream(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = client.Close() }()

	pub := NewEnrichPublisher(client)
	ctx := context.Background()

	// Seed DLQ with 10 entries
	for i := range 10 {
		req := EnrichRequest{
			Token:      string(rune('a' + i)),
			Priority:   1,
			Source:     "dlq-test",
			EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: EnrichDeadLetterStream,
			Values: map[string]any{"data": string(data)},
		}).Err(); err != nil {
			t.Fatalf("xadd %d: %v", i, err)
		}
	}

	// Verify 10 entries
	len1, err := client.XLen(ctx, EnrichDeadLetterStream).Result()
	if err != nil {
		t.Fatalf("xlen before: %v", err)
	}
	if len1 != 10 {
		t.Fatalf("expected 10 entries before trim, got %d", len1)
	}

	// Trim to 5
	if err := pub.TrimDeadLetterStream(ctx, 5); err != nil {
		t.Fatalf("trim: %v", err)
	}

	// Verify 5 entries remain
	len2, err := client.XLen(ctx, EnrichDeadLetterStream).Result()
	if err != nil {
		t.Fatalf("xlen after: %v", err)
	}
	if len2 != 5 {
		t.Errorf("expected 5 entries after trim, got %d", len2)
	}
}

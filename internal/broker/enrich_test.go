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

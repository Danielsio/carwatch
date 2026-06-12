package scheduler

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/dsionov/carwatch/internal/broker"
)

func backfillTestScheduler(t *testing.T, tokens []string) (*Scheduler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	s := &Scheduler{
		logger:          testLogger(),
		enrichPublisher: broker.NewEnrichPublisher(client),
		stores:          Stores{Listings: &mockListingStore{unenrichedTokens: tokens}},
	}
	return s, mr
}

// TestBackfill_PublishesBelowWatermark verifies the normal path: with a shallow
// stream, every unenriched token is published.
func TestBackfill_PublishesBelowWatermark(t *testing.T) {
	s, mr := backfillTestScheduler(t, []string{"tok1", "tok2", "tok3"})

	s.backfillUnenrichedListings(context.Background())

	if got := streamLen(t, mr); got != 3 {
		t.Fatalf("expected 3 published backfill requests, stream len = %d", got)
	}
}

func streamLen(t *testing.T, mr *miniredis.Miniredis) int {
	t.Helper()
	entries, err := mr.Stream(broker.EnrichStreamName)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	return len(entries)
}

// TestBackfill_SkipsAboveWatermark verifies the backpressure path: when the
// enrich stream is at/above the watermark, backfill publishes nothing — the
// stream is XAdd-trimmed at MaxLen, so publishing into a near-full stream
// would silently evict in-flight work (F5).
func TestBackfill_SkipsAboveWatermark(t *testing.T) {
	s, mr := backfillTestScheduler(t, []string{"tok1", "tok2"})

	// Seed the stream directly to the watermark via miniredis (in-process, fast).
	for i := 0; i < enrichBackfillWatermark; i++ {
		if _, err := mr.XAdd(broker.EnrichStreamName, "*", []string{"data", "x"}); err != nil {
			t.Fatalf("seed stream: %v", err)
		}
	}

	s.backfillUnenrichedListings(context.Background())

	if got := streamLen(t, mr); got != enrichBackfillWatermark {
		t.Fatalf("expected no publishes above watermark, stream len = %d (watermark %d)",
			got, enrichBackfillWatermark)
	}
}

// TestBackfill_SkipsOnDepthCheckError verifies that an unreachable Redis means
// no backfill attempt (publishing would fail anyway; skip quietly).
func TestBackfill_SkipsOnDepthCheckError(t *testing.T) {
	s, mr := backfillTestScheduler(t, []string{"tok1"})
	mr.Close() // make XLen fail

	// Must not panic and must not publish.
	s.backfillUnenrichedListings(context.Background())
}

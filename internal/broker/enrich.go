package broker

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"
)

// EnrichStreamName is the Redis Stream key for enrichment requests.
const EnrichStreamName = "carwatch:enrich"

// EnrichGroupName is the consumer group for enrichment workers.
const EnrichGroupName = "enricher-workers"

// EnrichDeadLetterStream holds messages that exceeded retry limits.
const EnrichDeadLetterStream = "carwatch:enrich:dead"

// EnrichPendingSet is the Redis SET key that tracks tokens currently pending enrichment.
const EnrichPendingSet = "carwatch:enrich:pending"

// EnrichStreamMaxLen caps the enrichment stream via approximate XAdd trimming.
// When the stream exceeds this, the OLDEST entries are silently evicted —
// producers must therefore check EnrichQueueLen before bulk publishing (see the
// scheduler backfill watermark).
const EnrichStreamMaxLen = 50000

// EnrichRequest represents a request to enrich a listing with km/city data.
type EnrichRequest struct {
	Token      string  `json:"token"`
	Priority   int     `json:"priority"`
	SearchIDs  []int64 `json:"search_ids,omitempty"`
	Source     string  `json:"source"`
	EnqueuedAt string  `json:"enqueued_at"`
}

// EnrichPublisher writes enrichment requests to the Redis Stream.
type EnrichPublisher struct {
	client *redis.Client
}

// NewEnrichPublisher creates an EnrichPublisher using an existing Redis client.
func NewEnrichPublisher(client *redis.Client) *EnrichPublisher {
	return &EnrichPublisher{client: client}
}

// PublishEnrich adds an enrichment request to the stream.
func (p *EnrichPublisher) PublishEnrich(ctx context.Context, req EnrichRequest) error {
	if p == nil || p.client == nil {
		return errors.New("enrich publisher not initialized")
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: EnrichStreamName,
		MaxLen: EnrichStreamMaxLen,
		Approx: true,
		Values: map[string]any{"data": string(data)},
	}).Err()
}

// PublishEnrichDedup adds an enrichment request to the stream only if the token
// is not already pending. It returns (true, nil) if published, (false, nil) if
// skipped due to deduplication, or (false, error) on failure.
func (p *EnrichPublisher) PublishEnrichDedup(ctx context.Context, req EnrichRequest) (bool, error) {
	if p == nil || p.client == nil {
		return false, errors.New("enrich publisher not initialized")
	}

	// Check if token is already pending
	isPending, err := p.client.SIsMember(ctx, EnrichPendingSet, req.Token).Result()
	if err != nil {
		return false, err
	}
	if isPending {
		return false, nil
	}

	// Marshal and publish to stream
	data, err := json.Marshal(req)
	if err != nil {
		return false, err
	}
	if err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: EnrichStreamName,
		MaxLen: EnrichStreamMaxLen,
		Approx: true,
		Values: map[string]any{"data": string(data)},
	}).Err(); err != nil {
		return false, err
	}

	// Add token to pending set
	if err := p.client.SAdd(ctx, EnrichPendingSet, req.Token).Err(); err != nil {
		return false, err
	}

	return true, nil
}

// RemovePending removes a token from the pending set.
func (p *EnrichPublisher) RemovePending(ctx context.Context, token string) error {
	if p == nil || p.client == nil {
		return errors.New("enrich publisher not initialized")
	}
	return p.client.SRem(ctx, EnrichPendingSet, token).Err()
}

// EnrichQueueLen returns the current length of the enrichment stream.
func (p *EnrichPublisher) EnrichQueueLen(ctx context.Context) (int64, error) {
	if p == nil || p.client == nil {
		return 0, errors.New("enrich publisher not initialized")
	}
	return p.client.XLen(ctx, EnrichStreamName).Result()
}

package broker

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// EnrichStreamName is the Redis Stream key for enrichment requests.
const EnrichStreamName = "carwatch:enrich"

// EnrichGroupName is the consumer group for enrichment workers.
const EnrichGroupName = "enricher-workers"

// EnrichDeadLetterStream holds messages that exceeded retry limits.
const EnrichDeadLetterStream = "carwatch:enrich:dead"

const enrichStreamMaxLen = 50000

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
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: EnrichStreamName,
		MaxLen: enrichStreamMaxLen,
		Approx: true,
		Values: map[string]any{"data": string(data)},
	}).Err()
}

// EnrichQueueLen returns the current length of the enrichment stream.
func (p *EnrichPublisher) EnrichQueueLen(ctx context.Context) (int64, error) {
	return p.client.XLen(ctx, EnrichStreamName).Result()
}

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

// PurgeEnrichedEntries reads the stream in batches and removes entries
// whose tokens the checker reports as fully enriched. Returns the count
// of purged entries.
func (p *EnrichPublisher) PurgeEnrichedEntries(ctx context.Context, checker func(ctx context.Context, tokens []string) (map[string]bool, error)) (int, error) {
	if p == nil || p.client == nil {
		return 0, errors.New("enrich publisher not initialized")
	}
	if checker == nil {
		return 0, errors.New("checker function is required")
	}

	const batchSize = 200
	purged := 0
	lastID := "-"

	for {
		// Read batch from stream
		msgs, err := p.client.XRange(ctx, EnrichStreamName, lastID, "+").Result()
		if err != nil {
			return purged, err
		}
		if len(msgs) == 0 {
			break
		}

		// If lastID was inclusive, skip the first message to avoid reprocessing
		if lastID != "-" && len(msgs) > 0 && msgs[0].ID == lastID {
			msgs = msgs[1:]
		}

		// Take up to batchSize messages
		if len(msgs) > batchSize {
			msgs = msgs[:batchSize]
		}
		if len(msgs) == 0 {
			break
		}

		// Extract tokens from batch
		tokens := make([]string, 0, len(msgs))
		msgMap := make(map[string]string) // token -> msgID
		for _, msg := range msgs {
			data, ok := msg.Values["data"].(string)
			if !ok {
				continue
			}
			var req EnrichRequest
			if err := json.Unmarshal([]byte(data), &req); err != nil {
				continue
			}
			tokens = append(tokens, req.Token)
			msgMap[req.Token] = msg.ID
		}

		// Check which tokens are fully enriched
		enrichedMap, err := checker(ctx, tokens)
		if err != nil {
			return purged, err
		}

		// Delete enriched entries from stream and pending set
		for token, isEnriched := range enrichedMap {
			if isEnriched {
				msgID, exists := msgMap[token]
				if !exists {
					continue
				}
				// XDEL the message
				if err := p.client.XDel(ctx, EnrichStreamName, msgID).Err(); err != nil {
					return purged, err
				}
				// SREM from pending set
				if err := p.client.SRem(ctx, EnrichPendingSet, token).Err(); err != nil {
					return purged, err
				}
				purged++
			}
		}

		// Move to next batch
		lastID = msgs[len(msgs)-1].ID
	}

	return purged, nil
}

// TrimDeadLetterStream caps the dead-letter stream to maxLen entries.
func (p *EnrichPublisher) TrimDeadLetterStream(ctx context.Context, maxLen int64) error {
	if p == nil || p.client == nil {
		return errors.New("enrich publisher not initialized")
	}
	return p.client.XTrimMaxLen(ctx, EnrichDeadLetterStream, maxLen).Err()
}

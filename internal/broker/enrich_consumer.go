package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	enrichMaxRetries        = 5
	enrichDeadLetterMaxLen  = 10000
	enrichOrphanIdleDefault = 5 * time.Minute
)

// EnrichFunc processes a single enrichment request. It should return nil
// on success (or skip), and an error to leave the message pending for retry.
type EnrichFunc func(ctx context.Context, req EnrichRequest) error

// EnrichConsumer reads enrichment requests from the carwatch:enrich
// Redis Stream, sorts each batch by priority, and processes them via
// a caller-provided EnrichFunc.
type EnrichConsumer struct {
	client   *redis.Client
	enrich   EnrichFunc
	logger   *slog.Logger
	consumer string
}

// NewEnrichConsumer connects to Redis, creates the consumer group if
// needed, and returns a consumer ready to process enrichment requests.
func NewEnrichConsumer(addr, password string, db int, fn EnrichFunc, logger *slog.Logger) (*EnrichConsumer, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	err := client.XGroupCreateMkStream(ctx, EnrichStreamName, EnrichGroupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("enricher-%d", time.Now().UnixNano())
	}
	return &EnrichConsumer{
		client:   client,
		enrich:   fn,
		logger:   logger,
		consumer: hostname,
	}, nil
}

// Run reads and processes enrichment requests until the context is cancelled.
func (c *EnrichConsumer) Run(ctx context.Context) error {
	reclaimTicker := time.NewTicker(30 * time.Second)
	defer reclaimTicker.Stop()

	orphanTicker := time.NewTicker(60 * time.Second)
	defer orphanTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-reclaimTicker.C:
			c.reclaimPending(ctx)
		case <-orphanTicker.C:
			c.reclaimOrphans(ctx)
		default:
		}

		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    EnrichGroupName,
			Consumer: c.consumer,
			Streams:  []string{EnrichStreamName, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Error("read from enrich stream failed", "error", err)
			time.Sleep(time.Second)
			continue
		}

		for _, stream := range streams {
			c.processBatch(ctx, stream.Messages)
		}
	}
}

// processBatch sorts messages by priority and processes them sequentially.
func (c *EnrichConsumer) processBatch(ctx context.Context, msgs []redis.XMessage) {
	type parsed struct {
		msg redis.XMessage
		req EnrichRequest
	}

	var items []parsed
	for _, msg := range msgs {
		data, ok := msg.Values["data"].(string)
		if !ok {
			c.ack(ctx, msg.ID)
			continue
		}
		var req EnrichRequest
		if err := json.Unmarshal([]byte(data), &req); err != nil {
			c.logger.Error("unmarshal enrich request", "id", msg.ID, "error", err)
			c.ack(ctx, msg.ID)
			continue
		}
		items = append(items, parsed{msg: msg, req: req})
	}

	// Lower numeric priority = higher urgency (1=match > 2=recent > 3=backfill).
	slices.SortFunc(items, func(a, b parsed) int {
		return a.req.Priority - b.req.Priority
	})

	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		if err := c.enrich(ctx, item.req); err != nil {
			c.logger.Warn("enrich request failed, will retry",
				"token", item.req.Token, "priority", item.req.Priority, "error", err)
			continue
		}
		c.ack(ctx, item.msg.ID)
	}
}

func (c *EnrichConsumer) reclaimPending(ctx context.Context) {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   EnrichStreamName,
		Group:    EnrichGroupName,
		Consumer: c.consumer,
		Start:    "-",
		End:      "+",
		Count:    50,
		Idle:     30 * time.Second,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}

	c.logger.Info("reclaiming pending enrich messages", "count", len(pending))
	for _, p := range pending {
		if p.RetryCount >= int64(enrichMaxRetries) {
			c.deadLetter(ctx, p.ID)
			continue
		}
		msgs, err := c.client.XRangeN(ctx, EnrichStreamName, p.ID, p.ID, 1).Result()
		if err != nil || len(msgs) == 0 {
			continue
		}
		c.processBatch(ctx, msgs)
	}
}

func (c *EnrichConsumer) deadLetter(ctx context.Context, id string) {
	msgs, err := c.client.XRangeN(ctx, EnrichStreamName, id, id, 1).Result()
	if err != nil {
		c.logger.Error("read enrich message for dead-letter failed", "id", id, "error", err)
		return
	}
	if len(msgs) == 0 {
		c.ack(ctx, id)
		return
	}
	if err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: EnrichDeadLetterStream,
		MaxLen: enrichDeadLetterMaxLen,
		Approx: true,
		Values: msgs[0].Values,
	}).Err(); err != nil {
		c.logger.Error("enrich dead-letter failed, leaving message pending", "id", id, "error", err)
		return
	}
	c.logger.Warn("enrich message dead-lettered after max retries",
		"id", id, "max_retries", enrichMaxRetries)
	c.ack(ctx, id)
}

func (c *EnrichConsumer) reclaimOrphans(ctx context.Context) {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: EnrichStreamName,
		Group:  EnrichGroupName,
		Start:  "-",
		End:    "+",
		Count:  50,
		Idle:   enrichOrphanIdleDefault,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}

	var orphanIDs []string
	for _, p := range pending {
		if p.Consumer == c.consumer {
			continue
		}
		if p.RetryCount >= int64(enrichMaxRetries) {
			c.deadLetter(ctx, p.ID)
			continue
		}
		orphanIDs = append(orphanIDs, p.ID)
	}
	if len(orphanIDs) == 0 {
		return
	}

	c.logger.Info("reclaiming orphaned enrich messages", "count", len(orphanIDs))

	claimed, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   EnrichStreamName,
		Group:    EnrichGroupName,
		Consumer: c.consumer,
		MinIdle:  enrichOrphanIdleDefault,
		Messages: orphanIDs,
	}).Result()
	if err != nil {
		c.logger.Error("xclaim enrich orphans failed", "error", err)
		return
	}

	c.processBatch(ctx, claimed)
}

// Drain processes any pending (claimed but unacked) messages before shutdown.
func (c *EnrichConsumer) Drain(ctx context.Context) {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   EnrichStreamName,
		Group:    EnrichGroupName,
		Consumer: c.consumer,
		Start:    "-",
		End:      "+",
		Count:    100,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}
	c.logger.Info("draining pending enrich messages", "count", len(pending))
	for _, p := range pending {
		msgs, err := c.client.XRangeN(ctx, EnrichStreamName, p.ID, p.ID, 1).Result()
		if err != nil || len(msgs) == 0 {
			continue
		}
		c.processBatch(ctx, msgs)
	}
}

// Close shuts down the Redis connection.
func (c *EnrichConsumer) Close() error { return c.client.Close() }

func (c *EnrichConsumer) ack(ctx context.Context, id string) {
	if err := c.client.XAck(ctx, EnrichStreamName, EnrichGroupName, id).Err(); err != nil {
		c.logger.Error("enrich ack failed", "id", id, "error", err)
	}
}

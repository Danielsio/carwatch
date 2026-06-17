package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"strconv"
	"strings"

	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/telemetry"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

const (
	// GroupName is the consumer group for notification workers.
	GroupName = "notifier-workers"
	// DeadLetterStream receives alerts that exceeded MaxRetries.
	DeadLetterStream = "carwatch:alerts:dead"
	// MaxRetries is the maximum number of delivery attempts before dead-lettering.
	MaxRetries = 3
)

// NotifyFunc delivers a single notification to a recipient.
type NotifyFunc func(ctx context.Context, recipient string, message string) error
type DeadLetterHook func(ctx context.Context, alert Alert) error

// DeliveryHook records the outcome of a delivery attempt for the persisted
// delivery ledger. msgID is the Redis stream id (used as a batch id grouping
// one Telegram message ↔ N listings). It is best-effort: it must not block
// acking, and errors are the hook's own responsibility to log.
type DeliveryHook func(ctx context.Context, alert Alert, msgID string, status string)

// Consumer reads alerts from the Redis Stream and delivers them.
type Consumer struct {
	client               *redis.Client
	notify               NotifyFunc
	deadLetterHook       DeadLetterHook
	deliveryHook         DeliveryHook
	limiter              *rate.Limiter
	logger               *slog.Logger
	consumer             string
	reclaimIdleThreshold time.Duration
	orphanIdleThreshold  time.Duration
}

// NewConsumer connects to Redis, creates the consumer group if needed,
// and returns a Consumer ready to process alerts.
func NewConsumer(addr, password string, db int, notify NotifyFunc, logger *slog.Logger, opts ...func(*Consumer)) (*Consumer, error) {
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

	// Create consumer group if not exists.
	err := client.XGroupCreateMkStream(ctx, StreamName, GroupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}
	c := &Consumer{
		client:               client,
		notify:               notify,
		limiter:              rate.NewLimiter(rate.Limit(30), 30), // 30 msg/sec Telegram limit
		logger:               logger,
		consumer:             hostname,
		reclaimIdleThreshold: 30 * time.Second,
		orphanIdleThreshold:  orphanIdleThreshold,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func WithDeadLetterHook(h DeadLetterHook) func(*Consumer) {
	return func(c *Consumer) {
		c.deadLetterHook = h
	}
}

func WithDeliveryHook(h DeliveryHook) func(*Consumer) {
	return func(c *Consumer) {
		c.deliveryHook = h
	}
}

func (c *Consumer) fireDelivery(ctx context.Context, alert Alert, msgID, status string) {
	if c.deliveryHook != nil {
		c.deliveryHook(ctx, alert, msgID, status)
	}
}

// Run reads and processes alerts in a loop until the context is cancelled.
// It alternates between reclaiming failed pending messages and reading new ones.
func (c *Consumer) Run(ctx context.Context) error {
	reclaimTicker := time.NewTicker(30 * time.Second)
	defer reclaimTicker.Stop()

	orphanTicker := time.NewTicker(60 * time.Second)
	defer orphanTicker.Stop()

	metricsTicker := time.NewTicker(30 * time.Second)
	defer metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-reclaimTicker.C:
			c.reclaimPending(ctx)
		case <-orphanTicker.C:
			c.reclaimOrphans(ctx)
		case <-metricsTicker.C:
			c.reportQueueMetrics(ctx)
		default:
		}

		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    GroupName,
			Consumer: c.consumer,
			Streams:  []string{StreamName, ">"},
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
			c.logger.Error("read from stream failed", "error", err)
			time.Sleep(time.Second)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				c.processMessage(ctx, msg)
			}
		}
	}
}

// reclaimPending reclaims messages that were delivered but not acked
// (failed deliveries). Messages exceeding MaxRetries are dead-lettered.
func (c *Consumer) reclaimPending(ctx context.Context) {
	idle := c.reclaimIdleThreshold
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   StreamName,
		Group:    GroupName,
		Consumer: c.consumer,
		Start:    "-",
		End:      "+",
		Count:    50,
		Idle:     idle,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}

	c.logger.Info("reclaiming pending messages", "count", len(pending))

	var retryIDs []string
	for _, p := range pending {
		if p.RetryCount >= int64(MaxRetries) {
			c.deadLetter(ctx, p.ID)
			continue
		}
		retryIDs = append(retryIDs, p.ID)
	}
	if len(retryIDs) == 0 {
		return
	}

	claimed, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   StreamName,
		Group:    GroupName,
		Consumer: c.consumer,
		MinIdle:  idle,
		Messages: retryIDs,
	}).Result()
	if err != nil {
		c.logger.Error("xclaim pending messages failed", "error", err)
		return
	}
	for _, msg := range claimed {
		c.processMessage(ctx, msg)
	}
}

// deadLetter moves a message to the dead-letter stream and acks it from the main stream.
func (c *Consumer) deadLetter(ctx context.Context, id string) {
	msgs, err := c.client.XRangeN(ctx, StreamName, id, id, 1).Result()
	if err != nil || len(msgs) == 0 {
		c.ack(ctx, id)
		return
	}
	alert, alertErr := parseAlert(msgs[0])
	if alertErr != nil {
		c.logger.Error("dead-letter alert parse failed", "id", id, "error", alertErr)
	}
	if c.deadLetterHook != nil && alertErr == nil {
		if err := c.deadLetterHook(ctx, alert); err != nil {
			c.logger.Error("dead-letter hook failed", "id", id, "chat_id", alert.ChatID, "search_name", alert.SearchName, "error", err)
			return
		}
	}
	dlAttrs := []any{"id", id, "max_retries", MaxRetries}
	if alertErr == nil {
		dlAttrs = append(dlAttrs, "chat_id", alert.ChatID, "search_name", alert.SearchName)
	}
	if err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: DeadLetterStream,
		MaxLen: 10000,
		Approx: true,
		Values: msgs[0].Values,
	}).Err(); err != nil {
		c.logger.Error("dead-letter failed", append(dlAttrs, "error", err)...)
		return
	} else {
		c.logger.Warn("message dead-lettered after max retries", dlAttrs...)
	}
	if alertErr == nil {
		c.fireDelivery(ctx, alert, id, "dead_lettered")
	}
	c.ack(ctx, id)
}

func (c *Consumer) processMessage(ctx context.Context, msg redis.XMessage) {
	alert, err := parseAlert(msg)
	if err != nil {
		c.logger.Error("unmarshal alert", "id", msg.ID, "error", err)
		c.ack(ctx, msg.ID)
		return
	}

	// Rate limit.
	if err := c.limiter.Wait(ctx); err != nil {
		return // context cancelled
	}

	recipient := fmt.Sprintf("%d", alert.ChatID)
	if err := c.notify(ctx, recipient, alert.Message); err != nil {
		if errors.Is(err, notifier.ErrNoChannelNotifier) || errors.Is(err, notifier.ErrRecipientBlocked) {
			if c.deadLetterHook != nil {
				if hookErr := c.deadLetterHook(ctx, alert); hookErr != nil {
					c.logger.Error("drop alert cleanup failed", "id", msg.ID, "chat_id", alert.ChatID, "error", hookErr)
					return
				}
			}
			c.logger.Warn("dropping alert for permanently undeliverable recipient",
				"id", msg.ID, "chat_id", alert.ChatID, "search_name", alert.SearchName, "error", err)
			c.ack(ctx, msg.ID)
			c.fireDelivery(ctx, alert, msg.ID, "dropped")
			return
		}
		if errors.Is(err, notifier.ErrNoDelivery) {
			// The recipient has no reachable target right now (e.g. a web user
			// with no push subscription). Retrying or re-claiming would loop
			// forever, so ack and keep the dedup claim — but log it so the
			// missed delivery is visible rather than a silent success.
			c.logger.Warn("no delivery target for recipient; acking without retry",
				"id", msg.ID, "chat_id", alert.ChatID, "search_name", alert.SearchName)
			c.ack(ctx, msg.ID)
			c.fireDelivery(ctx, alert, msg.ID, "dropped")
			return
		}
		c.logger.Error("deliver alert failed", "id", msg.ID, "chat_id", alert.ChatID, "search_name", alert.SearchName, "error", err)
		return
	}

	c.ack(ctx, msg.ID)
	c.logger.Info("alert delivered", "id", msg.ID, "chat_id", alert.ChatID, "search_name", alert.SearchName)
	c.fireDelivery(ctx, alert, msg.ID, "sent")

	if telemetry.QueueLag != nil {
		if lag := messageAge(msg.ID); lag > 0 {
			telemetry.QueueLag.Record(ctx, lag.Seconds())
		}
	}
}

func parseAlert(msg redis.XMessage) (Alert, error) {
	data, ok := msg.Values["data"].(string)
	if !ok {
		return Alert{}, fmt.Errorf("missing data field")
	}
	var alert Alert
	if err := json.Unmarshal([]byte(data), &alert); err != nil {
		return Alert{}, err
	}
	return alert, nil
}

func (c *Consumer) ack(ctx context.Context, id string) {
	if err := c.client.XAck(ctx, StreamName, GroupName, id).Err(); err != nil {
		c.logger.Error("ack failed", "id", id, "error", err)
	}
}

func (c *Consumer) reportQueueMetrics(ctx context.Context) {
	if telemetry.QueueDepth != nil {
		if depth, err := c.client.XLen(ctx, StreamName).Result(); err == nil {
			telemetry.QueueDepth.Record(ctx, depth)
		}
	}
	if telemetry.QueuePending != nil {
		if info, err := c.client.XPending(ctx, StreamName, GroupName).Result(); err == nil {
			telemetry.QueuePending.Record(ctx, info.Count)
		}
	}
}

func messageAge(id string) time.Duration {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) == 0 {
		return 0
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0
	}
	return time.Since(time.UnixMilli(ms))
}

const orphanIdleThreshold = 5 * time.Minute

// reclaimOrphans claims messages pending under other (possibly dead) consumers
// that have been idle for at least orphanIdleThreshold. This ensures messages
// aren't lost when a consumer crashes and restarts with a different hostname.
func (c *Consumer) reclaimOrphans(ctx context.Context) {
	idle := c.orphanIdleThreshold
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: StreamName,
		Group:  GroupName,
		Start:  "-",
		End:    "+",
		Count:  50,
		Idle:   idle,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}

	var orphanIDs []string
	for _, p := range pending {
		if p.Consumer == c.consumer {
			continue
		}
		if p.RetryCount >= int64(MaxRetries) {
			c.deadLetter(ctx, p.ID)
			continue
		}
		orphanIDs = append(orphanIDs, p.ID)
	}
	if len(orphanIDs) == 0 {
		return
	}

	c.logger.Info("reclaiming orphaned messages", "count", len(orphanIDs))

	claimed, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   StreamName,
		Group:    GroupName,
		Consumer: c.consumer,
		MinIdle:  idle,
		Messages: orphanIDs,
	}).Result()
	if err != nil {
		c.logger.Error("xclaim orphans failed", "error", err)
		return
	}

	for _, msg := range claimed {
		c.processMessage(ctx, msg)
	}
}

// Drain processes any pending (claimed but unacked) messages before shutdown.
func (c *Consumer) Drain(ctx context.Context) {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   StreamName,
		Group:    GroupName,
		Consumer: c.consumer,
		Start:    "-",
		End:      "+",
		Count:    100,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}
	c.logger.Info("draining pending messages", "count", len(pending))
	for _, p := range pending {
		if p.RetryCount >= int64(MaxRetries) {
			c.deadLetter(ctx, p.ID)
			continue
		}
		msgs, err := c.client.XRangeN(ctx, StreamName, p.ID, p.ID, 1).Result()
		if err != nil || len(msgs) == 0 {
			continue
		}
		c.processMessage(ctx, msgs[0])
	}
}

// Close shuts down the Redis connection.
func (c *Consumer) Close() error { return c.client.Close() }

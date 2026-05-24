package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

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

// Consumer reads alerts from the Redis Stream and delivers them.
type Consumer struct {
	client   *redis.Client
	notify   NotifyFunc
	limiter  *rate.Limiter
	logger   *slog.Logger
	consumer string
}

// NewConsumer connects to Redis, creates the consumer group if needed,
// and returns a Consumer ready to process alerts.
func NewConsumer(addr, password string, db int, notify NotifyFunc, logger *slog.Logger) (*Consumer, error) {
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
	return &Consumer{
		client:   client,
		notify:   notify,
		limiter:  rate.NewLimiter(rate.Limit(30), 30), // 30 msg/sec Telegram limit
		logger:   logger,
		consumer: hostname,
	}, nil
}

// Run reads and processes alerts in a loop until the context is cancelled.
// It alternates between reclaiming failed pending messages and reading new ones.
func (c *Consumer) Run(ctx context.Context) error {
	reclaimTicker := time.NewTicker(30 * time.Second)
	defer reclaimTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-reclaimTicker.C:
			c.reclaimPending(ctx)
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
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   StreamName,
		Group:    GroupName,
		Consumer: c.consumer,
		Start:    "-",
		End:      "+",
		Count:    50,
		Idle:     30 * time.Second,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}

	c.logger.Info("reclaiming pending messages", "count", len(pending))
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

// deadLetter moves a message to the dead-letter stream and acks it from the main stream.
func (c *Consumer) deadLetter(ctx context.Context, id string) {
	msgs, err := c.client.XRangeN(ctx, StreamName, id, id, 1).Result()
	if err != nil || len(msgs) == 0 {
		c.ack(ctx, id)
		return
	}
	if err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: DeadLetterStream,
		MaxLen: 10000,
		Approx: true,
		Values: msgs[0].Values,
	}).Err(); err != nil {
		c.logger.Error("dead-letter failed", "id", id, "error", err)
	} else {
		c.logger.Warn("message dead-lettered after max retries", "id", id, "max_retries", MaxRetries)
	}
	c.ack(ctx, id)
}

func (c *Consumer) processMessage(ctx context.Context, msg redis.XMessage) {
	data, ok := msg.Values["data"].(string)
	if !ok {
		c.ack(ctx, msg.ID)
		return
	}

	var alert Alert
	if err := json.Unmarshal([]byte(data), &alert); err != nil {
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
		c.logger.Error("deliver alert failed", "id", msg.ID, "chat_id", alert.ChatID, "error", err)
		// Don't ack -- will be retried on next read of pending.
		return
	}

	c.ack(ctx, msg.ID)
	c.logger.Debug("alert delivered", "id", msg.ID, "chat_id", alert.ChatID, "search_name", alert.SearchName)
}

func (c *Consumer) ack(ctx context.Context, id string) {
	if err := c.client.XAck(ctx, StreamName, GroupName, id).Err(); err != nil {
		c.logger.Error("ack failed", "id", id, "error", err)
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
		msgs, err := c.client.XRangeN(ctx, StreamName, p.ID, p.ID, 1).Result()
		if err != nil || len(msgs) == 0 {
			continue
		}
		c.processMessage(ctx, msgs[0])
	}
}

// Close shuts down the Redis connection.
func (c *Consumer) Close() error { return c.client.Close() }

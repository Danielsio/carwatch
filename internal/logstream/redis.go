package logstream

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const logChannel = "carwatch:logs"

// RedisPublisher publishes LogEntry values to a Redis pub/sub channel.
type RedisPublisher struct {
	client *redis.Client
}

// NewRedisPublisher creates a publisher that sends log entries to Redis.
func NewRedisPublisher(addr, password string, db int) (*RedisPublisher, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RedisPublisher{client: client}, nil
}

// Publish sends a log entry to the Redis pub/sub channel.
func (p *RedisPublisher) Publish(e LogEntry) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	p.client.Publish(context.Background(), logChannel, string(data))
}

// Close shuts down the Redis connection.
func (p *RedisPublisher) Close() error { return p.client.Close() }

// RedisSubscriber reads log entries from a Redis pub/sub channel and
// feeds them into a Hub for SSE streaming.
type RedisSubscriber struct {
	client *redis.Client
	cancel context.CancelFunc
}

// NewRedisSubscriber subscribes to the log channel and publishes
// received entries to the given hub. Call Close to stop.
func NewRedisSubscriber(addr, password string, db int, hub *Hub, logger *slog.Logger) (*RedisSubscriber, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithCancel(context.Background())

	pubsub := client.Subscribe(ctx, logChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		cancel()
		_ = client.Close()
		return nil, err
	}

	go func() {
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var entry LogEntry
				if err := json.Unmarshal([]byte(msg.Payload), &entry); err != nil {
					logger.Debug("redis log subscriber: unmarshal failed", "error", err)
					continue
				}
				hub.Publish(entry)
			}
		}
	}()

	return &RedisSubscriber{client: client, cancel: cancel}, nil
}

// Close stops the subscriber and shuts down the Redis connection.
func (s *RedisSubscriber) Close() error {
	s.cancel()
	return s.client.Close()
}

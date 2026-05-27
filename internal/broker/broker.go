package broker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// Alert represents a notification to be delivered via Redis Streams.
type Alert struct {
	ChatID     int64    `json:"chat_id"`
	SearchID   int64    `json:"search_id"`
	SearchName string   `json:"search_name"`
	Tokens     []string `json:"tokens,omitempty"`
	Message    string   `json:"message"`
	Language   string   `json:"language"`
	Timestamp  string   `json:"timestamp"`
}

// StreamName is the Redis Stream key for alert delivery.
const StreamName = "carwatch:alerts"

// Publisher writes alerts to the Redis Stream.
type Publisher struct {
	client *redis.Client
}

// NewPublisher connects to Redis and returns a Publisher.
func NewPublisher(addr, password string, db int) (*Publisher, error) {
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
	return &Publisher{client: client}, nil
}

// Publish adds an alert to the Redis Stream.
func (p *Publisher) Publish(ctx context.Context, alert Alert) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamName,
		MaxLen: 100000,
		Approx: true,
		Values: map[string]any{"data": string(data)},
	}).Err()
}

// Client returns the underlying Redis client so it can be shared with other
// publishers (e.g. EnrichPublisher) without opening a second connection.
func (p *Publisher) Client() *redis.Client { return p.client }

// Close shuts down the Redis connection.
func (p *Publisher) Close() error { return p.client.Close() }

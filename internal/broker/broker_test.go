package broker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/redis/go-redis/v9"
)

func TestAlertSerializationRoundtrip(t *testing.T) {
	original := Alert{
		ChatID:     12345,
		SearchID:   42,
		SearchName: "Toyota Corolla 2021",
		Tokens:     []string{"tok-1", "tok-2"},
		Message:    "New listing found!",
		Language:   "he",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Alert
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ChatID != original.ChatID {
		t.Errorf("ChatID = %d, want %d", decoded.ChatID, original.ChatID)
	}
	if decoded.SearchID != original.SearchID {
		t.Errorf("SearchID = %d, want %d", decoded.SearchID, original.SearchID)
	}
	if decoded.SearchName != original.SearchName {
		t.Errorf("SearchName = %q, want %q", decoded.SearchName, original.SearchName)
	}
	if len(decoded.Tokens) != len(original.Tokens) {
		t.Fatalf("Tokens len = %d, want %d", len(decoded.Tokens), len(original.Tokens))
	}
	for i := range original.Tokens {
		if decoded.Tokens[i] != original.Tokens[i] {
			t.Fatalf("Tokens[%d] = %q, want %q", i, decoded.Tokens[i], original.Tokens[i])
		}
	}
	if decoded.Message != original.Message {
		t.Errorf("Message = %q, want %q", decoded.Message, original.Message)
	}
	if decoded.Language != original.Language {
		t.Errorf("Language = %q, want %q", decoded.Language, original.Language)
	}
	if decoded.Timestamp != original.Timestamp {
		t.Errorf("Timestamp = %q, want %q", decoded.Timestamp, original.Timestamp)
	}
}

func TestPublishAndConsume(t *testing.T) {
	mr := miniredis.RunT(t)

	pub, err := NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	alert := Alert{
		ChatID:     999,
		SearchID:   7,
		SearchName: "Test Search",
		Message:    "Hello from Redis Streams!",
		Language:   "en",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	ctx := context.Background()
	if err := pub.Publish(ctx, alert); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Verify the message is in the stream.
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	msgs, err := client.XRange(ctx, StreamName, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	data, ok := msgs[0].Values["data"].(string)
	if !ok {
		t.Fatal("data field missing or not a string")
	}

	var got Alert
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ChatID != alert.ChatID {
		t.Errorf("ChatID = %d, want %d", got.ChatID, alert.ChatID)
	}
	if got.Message != alert.Message {
		t.Errorf("Message = %q, want %q", got.Message, alert.Message)
	}
}

func TestConsumerDeliversAndAcks(t *testing.T) {
	mr := miniredis.RunT(t)

	// Publish an alert.
	pub, err := NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	alert := Alert{
		ChatID:  42,
		Message: "Test delivery message!",
	}
	if err := pub.Publish(context.Background(), alert); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Track delivered messages.
	var mu sync.Mutex
	var delivered []struct {
		recipient string
		message   string
	}

	notify := func(_ context.Context, recipient string, message string) error {
		mu.Lock()
		defer mu.Unlock()
		delivered = append(delivered, struct {
			recipient string
			message   string
		}{recipient, message})
		return nil
	}

	logger := slog.Default()
	cons, err := NewConsumer(mr.Addr(), "", 0, notify, logger)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Run consumer in background.
	done := make(chan error, 1)
	go func() {
		done <- cons.Run(ctx)
	}()

	// Wait for delivery.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(delivered)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for delivery")
		case <-time.After(50 * time.Millisecond):
		}
	}

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(delivered))
	}
	if delivered[0].recipient != "42" {
		t.Errorf("recipient = %q, want '42'", delivered[0].recipient)
	}
	if delivered[0].message != "Test delivery message!" {
		t.Errorf("message = %q, want 'Test delivery message!'", delivered[0].message)
	}
}

func TestConsumerReclaimsOrphanedMessages(t *testing.T) {
	mr := miniredis.RunT(t)

	pub, err := NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	alert := Alert{ChatID: 42, Message: "orphan test message"}
	if err := pub.Publish(context.Background(), alert); err != nil {
		t.Fatalf("publish: %v", err)
	}

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	// Create the consumer group (normally done by NewConsumer).
	if err := client.XGroupCreateMkStream(ctx, StreamName, GroupName, "0").Err(); err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		t.Fatalf("create consumer group: %v", err)
	}

	// Read the message as "dead-consumer" to create a PEL entry, then abandon it.
	_, err = client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    GroupName,
		Consumer: "dead-consumer",
		Streams:  []string{StreamName, ">"},
		Count:    10,
		Block:    time.Second,
	}).Result()
	if err != nil {
		t.Fatalf("xreadgroup as dead-consumer: %v", err)
	}

	// Create the real consumer.
	var mu sync.Mutex
	var delivered []string
	notify := func(_ context.Context, _ string, message string) error {
		mu.Lock()
		defer mu.Unlock()
		delivered = append(delivered, message)
		return nil
	}

	cons, err := NewConsumer(mr.Addr(), "", 0, notify, slog.Default())
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	// Use zero idle threshold so the test doesn't need real time to pass.
	cons.orphanIdleThreshold = 0

	cons.reclaimOrphans(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 1 {
		t.Errorf("expected 1 orphaned message reclaimed, got %d", len(delivered))
	}
	if len(delivered) > 0 && delivered[0] != "orphan test message" {
		t.Errorf("message = %q, want %q", delivered[0], "orphan test message")
	}
}

func TestPublisherConnectionFailure(t *testing.T) {
	_, err := NewPublisher("localhost:1", "", 0)
	if err == nil {
		t.Fatal("expected error for unreachable Redis")
	}
}

func TestConsumerConnectionFailure(t *testing.T) {
	notify := func(_ context.Context, _ string, _ string) error { return nil }
	_, err := NewConsumer("localhost:1", "", 0, notify, slog.Default())
	if err == nil {
		t.Fatal("expected error for unreachable Redis")
	}
}

func TestConsumerAcksUnsupportedChannelWithoutRetry(t *testing.T) {
	mr := miniredis.RunT(t)

	pub, err := NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	alert := Alert{ChatID: 777, Message: "unsupported channel"}
	if err := pub.Publish(context.Background(), alert); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var calls atomic.Int32
	notify := func(_ context.Context, _ string, _ string) error {
		calls.Add(1)
		return notifier.ErrNoChannelNotifier
	}
	cons, err := NewConsumer(mr.Addr(), "", 0, notify, slog.Default())
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	_ = cons.Run(ctx)

	cons.reclaimPending(context.Background())

	if got := calls.Load(); got != 1 {
		t.Fatalf("notify calls = %d, want 1", got)
	}

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()
	pending, err := client.XPending(context.Background(), StreamName, GroupName).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("expected 0 pending entries, got %d", pending.Count)
	}
}

func TestPublisherWithRedisAuth(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("secret")

	pub, err := NewPublisher(mr.Addr(), "secret", 0)
	if err != nil {
		t.Fatalf("new publisher with auth: %v", err)
	}
	defer func() { _ = pub.Close() }()
}

func TestPublisherWithRedisAuthWrongPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("secret")

	_, err := NewPublisher(mr.Addr(), "wrong", 0)
	if err == nil {
		t.Fatal("expected auth error for wrong Redis password")
	}
}

func TestConsumerWithRedisAuth(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("secret")

	notify := func(_ context.Context, _ string, _ string) error { return nil }
	cons, err := NewConsumer(mr.Addr(), "secret", 0, notify, slog.Default())
	if err != nil {
		t.Fatalf("new consumer with auth: %v", err)
	}
	defer func() { _ = cons.Close() }()
}

func TestConsumerWithRedisAuthWrongPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("secret")

	notify := func(_ context.Context, _ string, _ string) error { return nil }
	_, err := NewConsumer(mr.Addr(), "wrong", 0, notify, slog.Default())
	if err == nil {
		t.Fatal("expected auth error for wrong Redis password")
	}
}

func TestPublishAndConsumeWithRedisAuth(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("secret")

	pub, err := NewPublisher(mr.Addr(), "secret", 0)
	if err != nil {
		t.Fatalf("new publisher with auth: %v", err)
	}
	defer func() { _ = pub.Close() }()

	alert := Alert{
		ChatID:   123,
		Message:  "auth-protected message",
		Language: "en",
	}
	if err := pub.Publish(context.Background(), alert); err != nil {
		t.Fatalf("publish with auth: %v", err)
	}

	var mu sync.Mutex
	var delivered []string
	notify := func(_ context.Context, _ string, message string) error {
		mu.Lock()
		defer mu.Unlock()
		delivered = append(delivered, message)
		return nil
	}
	cons, err := NewConsumer(mr.Addr(), "secret", 0, notify, slog.Default())
	if err != nil {
		t.Fatalf("new consumer with auth: %v", err)
	}
	defer func() { _ = cons.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cons.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(delivered)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for authenticated delivery")
		case <-time.After(50 * time.Millisecond):
		}
	}

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(delivered))
	}
	if delivered[0] != "auth-protected message" {
		t.Fatalf("message = %q, want %q", delivered[0], "auth-protected message")
	}
}

func TestDeadLetterCallsHookAndAcks(t *testing.T) {
	mr := miniredis.RunT(t)
	pub, err := NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	alert := Alert{
		ChatID:   42,
		SearchID: 7,
		Tokens:   []string{"abc", "def"},
		Message:  "msg",
	}
	if err := pub.Publish(context.Background(), alert); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var got Alert
	notify := func(_ context.Context, _ string, _ string) error { return nil }
	cons, err := NewConsumer(mr.Addr(), "", 0, notify, slog.Default(), WithDeadLetterHook(func(_ context.Context, a Alert) error {
		got = a
		return nil
	}))
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()
	msgs, err := client.XRange(context.Background(), StreamName, "-", "+").Result()
	if err != nil || len(msgs) != 1 {
		t.Fatalf("xrange: %v len=%d", err, len(msgs))
	}
	cons.deadLetter(context.Background(), msgs[0].ID)

	if got.ChatID != 42 || len(got.Tokens) != 2 {
		t.Fatalf("unexpected hook alert: %+v", got)
	}
	deadMsgs, err := client.XRange(context.Background(), DeadLetterStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("dead xrange: %v", err)
	}
	if len(deadMsgs) != 1 {
		t.Fatalf("expected 1 dead-letter message, got %d", len(deadMsgs))
	}
	pending, err := client.XPending(context.Background(), StreamName, GroupName).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("expected 0 pending after dead-letter ack, got %d", pending.Count)
	}
}

func TestDeadLetterHookFailureDoesNotAckOrMoveMessage(t *testing.T) {
	mr := miniredis.RunT(t)
	pub, err := NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()
	if err := pub.Publish(context.Background(), Alert{ChatID: 7, Tokens: []string{"x"}, Message: "msg"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	notify := func(_ context.Context, _ string, _ string) error { return nil }
	cons, err := NewConsumer(mr.Addr(), "", 0, notify, slog.Default(), WithDeadLetterHook(func(_ context.Context, _ Alert) error {
		return context.DeadlineExceeded
	}))
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	defer func() { _ = cons.Close() }()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()
	msgs, err := client.XRange(context.Background(), StreamName, "-", "+").Result()
	if err != nil || len(msgs) != 1 {
		t.Fatalf("xrange: %v len=%d", err, len(msgs))
	}
	// Create a pending entry so XPending can observe it.
	_, err = client.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group:    GroupName,
		Consumer: "hook-fail-consumer",
		Streams:  []string{StreamName, ">"},
		Count:    1,
		Block:    time.Second,
	}).Result()
	if err != nil {
		t.Fatalf("xreadgroup: %v", err)
	}

	cons.deadLetter(context.Background(), msgs[0].ID)

	deadMsgs, err := client.XRange(context.Background(), DeadLetterStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("dead xrange: %v", err)
	}
	if len(deadMsgs) != 0 {
		t.Fatalf("expected 0 dead-letter messages on hook failure, got %d", len(deadMsgs))
	}
	pending, err := client.XPending(context.Background(), StreamName, GroupName).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count == 0 {
		t.Fatalf("expected pending message to remain after hook failure")
	}
}

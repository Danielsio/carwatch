//go:build e2e

package e2e

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/pgtest"
)

func TestE2E_NotificationFlowViaRedisStream(t *testing.T) {
	store := pgtest.NewStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.UpsertUser(ctx, 100, "testuser"); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	searchID, err := store.CreateSearch(ctx, storage.Search{
		ChatID: 100, Name: "e2e-redis-flow", Source: "yad2",
		Manufacturer: 27, Model: 10332, YearMin: 2020, YearMax: 2024,
		PriceMax: 200000, Active: true,
	})
	if err != nil {
		t.Fatalf("create search: %v", err)
	}

	mr := miniredis.RunT(t)

	pub, err := broker.NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}

	var mu sync.Mutex
	var delivered []string

	consumer, err := broker.NewConsumer(mr.Addr(), "", 0,
		func(_ context.Context, recipient string, message string) error {
			mu.Lock()
			delivered = append(delivered, recipient+":"+message)
			mu.Unlock()
			return nil
		},
		testLogger,
	)
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	consumerCtx, consumerCancel := context.WithCancel(ctx)
	defer consumerCancel()
	go func() { _ = consumer.Run(consumerCtx) }()

	alert := broker.Alert{
		ChatID:     100,
		SearchID:   searchID,
		SearchName: "e2e-redis-flow",
		Tokens:     []string{"redis-tok-1"},
		Message:    "Mazda 3 2021 - 95,000 ₪",
		Language:   string(locale.Hebrew),
		Timestamp:  time.Now().Format(time.RFC3339),
	}
	if err := pub.Publish(ctx, alert); err != nil {
		t.Fatalf("publish alert: %v", err)
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for consumer to deliver notification")
		case <-time.After(100 * time.Millisecond):
			mu.Lock()
			n := len(delivered)
			mu.Unlock()
			if n > 0 {
				mu.Lock()
				got := delivered[0]
				mu.Unlock()
				if got == "" {
					t.Fatal("empty delivery")
				}
				t.Logf("delivered: %s", got)

				claimed, err := store.ClaimNew(ctx, "redis-tok-1", 100, searchID)
				if err != nil {
					t.Fatalf("claim: %v", err)
				}
				if claimed {
					t.Log("dedup claim succeeded (expected for first claim)")
				}

				claimed2, _ := store.ClaimNew(ctx, "redis-tok-1", 100, searchID)
				if claimed2 {
					t.Error("second claim should fail (dedup)")
				}
				return
			}
		}
	}
}


//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/dsionov/carwatch/internal/api"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/pgtest"
)

// TestE2E_ExtIngestNotifiesOncePerListing drives the browser-extension ingest
// path end to end against a real Postgres and a real (mini) Redis stream:
//
//	POST /api/v1/ext/ingest  →  filter+score+save  →  dedup (seen_listings)
//	→  publish alert  →  notifier consumer delivers.
//
// It asserts the new listing produces exactly one notification, and that
// re-ingesting the same listing on the next cycle does NOT re-notify — the
// dedup guarantee that keeps the extension from re-alerting every 15 minutes.
func TestE2E_ExtIngestNotifiesOncePerListing(t *testing.T) {
	store := pgtest.NewStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const chatID = int64(100)
	if err := store.UpsertUser(ctx, chatID, "ext-user"); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if _, err := store.CreateSearch(ctx, storage.Search{
		ChatID: chatID, Name: "e2e-ext-ingest", Source: "yad2",
		Manufacturer: 27, Model: 10332, YearMin: 2020, YearMax: 2024,
		PriceMax: 200000, MaxKm: 300000, MaxHand: 10, Active: true,
	}); err != nil {
		t.Fatalf("create search: %v", err)
	}

	// Real Redis stream + a consumer that records deliveries.
	mr := miniredis.RunT(t)
	pub, err := broker.NewPublisher(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	var mu sync.Mutex
	var delivered []string
	consumer, err := broker.NewConsumer(mr.Addr(), "", 0,
		func(_ context.Context, recipient, message string) error {
			mu.Lock()
			delivered = append(delivered, recipient+":"+message)
			mu.Unlock()
			return nil
		}, testLogger)
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	consumerCtx, consumerCancel := context.WithCancel(ctx)
	defer consumerCancel()
	go func() { _ = consumer.Run(consumerCtx) }()

	srv := api.New(api.Config{
		Searches:       store,
		Listings:       store,
		Users:          store,
		Prices:         store,
		Dedup:          store,
		Digests:        store,
		AlertPublisher: pub,
		Logger:         testLogger,
		API:            config.APIConfig{DevChatID: chatID, AuthToken: "test-token"},
	})

	deliveredCount := func() int { mu.Lock(); defer mu.Unlock(); return len(delivered) }

	ingest := func(token string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{
			"listings": []map[string]any{{
				"token":           token,
				"manufacturer":    "Mazda",
				"manufacturer_id": 27,
				"model":           "Mazda 3",
				"model_id":        10332,
				"year":            2022,
				"price":           95000,
				"km":              50000,
				"hand":            1,
				"city":            "Tel Aviv",
				"page_link":       "https://www.yad2.co.il/vehicles/item/" + token,
			}},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ext/ingest", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Routes().ServeHTTP(w, req)
		return w
	}

	// First ingest of a brand-new listing → one notification.
	w := ingest("ext-tok-1")
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Processed  int `json:"processed"`
		NewMatches int `json:"new_matches"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if resp.Processed < 1 {
		t.Fatalf("expected at least 1 processed listing, got %d", resp.Processed)
	}

	waitFor(t, 10*time.Second, func() bool { return deliveredCount() >= 1 })
	if got := deliveredCount(); got != 1 {
		t.Fatalf("expected exactly 1 notification after first ingest, got %d", got)
	}

	// Re-ingest the SAME listing (next extension cycle) → no new notification.
	if w := ingest("ext-tok-1"); w.Code != http.StatusOK {
		t.Fatalf("re-ingest: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Give any (erroneous) second alert time to flow through the stream.
	time.Sleep(2 * time.Second)
	if got := deliveredCount(); got != 1 {
		t.Fatalf("dedup failed: expected still 1 notification after re-ingest, got %d", got)
	}
}

// waitFor polls cond until true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if cond() {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for condition")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

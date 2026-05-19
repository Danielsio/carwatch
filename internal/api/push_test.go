package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPushSubscribe(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := subscribeRequest{
		Endpoint: "https://fcm.googleapis.com/push/sub1",
		Keys: subscribeKeys{
			P256DH: "BNcRd...",
			Auth:   "tBHI...",
		},
	}
	w := doRequest(t, srv, "POST", "/api/v1/push/subscribe", body)
	if w.Code != http.StatusNoContent {
		t.Fatalf("subscribe: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushSubscribe_MissingFields(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Missing endpoint
	w := doRequest(t, srv, "POST", "/api/v1/push/subscribe", subscribeRequest{
		Keys: subscribeKeys{P256DH: "key", Auth: "auth"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing endpoint: expected 400, got %d", w.Code)
	}

	// Missing p256dh
	w = doRequest(t, srv, "POST", "/api/v1/push/subscribe", subscribeRequest{
		Endpoint: "https://example.com/push",
		Keys:     subscribeKeys{Auth: "auth"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing p256dh: expected 400, got %d", w.Code)
	}

	// Missing auth
	w = doRequest(t, srv, "POST", "/api/v1/push/subscribe", subscribeRequest{
		Endpoint: "https://example.com/push",
		Keys:     subscribeKeys{P256DH: "key"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing auth: expected 400, got %d", w.Code)
	}
}

func TestPushUnsubscribe(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Subscribe first
	sub := subscribeRequest{
		Endpoint: "https://fcm.googleapis.com/push/del",
		Keys:     subscribeKeys{P256DH: "key", Auth: "auth"},
	}
	w := doRequest(t, srv, "POST", "/api/v1/push/subscribe", sub)
	if w.Code != http.StatusNoContent {
		t.Fatalf("subscribe: expected 204, got %d", w.Code)
	}

	// Unsubscribe
	w = doRequest(t, srv, "DELETE", "/api/v1/push/subscribe", unsubscribeRequest{
		Endpoint: "https://fcm.googleapis.com/push/del",
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("unsubscribe: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushUnsubscribe_MissingEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "DELETE", "/api/v1/push/subscribe", unsubscribeRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing endpoint: expected 400, got %d", w.Code)
	}
}

func TestPushVAPIDKey(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "GET", "/api/v1/push/vapid-key", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("vapid key: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["public_key"] != "test-vapid-public-key" {
		t.Fatalf("public_key = %q, want test-vapid-public-key", resp["public_key"])
	}
}

func TestPushSubscribe_Upsert(t *testing.T) {
	srv, store := setupTestServer(t)

	endpoint := "https://fcm.googleapis.com/push/upsert"

	// Subscribe
	w := doRequest(t, srv, "POST", "/api/v1/push/subscribe", subscribeRequest{
		Endpoint: endpoint,
		Keys:     subscribeKeys{P256DH: "old-key", Auth: "old-auth"},
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("subscribe: expected 204, got %d", w.Code)
	}

	// Upsert with new keys
	w = doRequest(t, srv, "POST", "/api/v1/push/subscribe", subscribeRequest{
		Endpoint: endpoint,
		Keys:     subscribeKeys{P256DH: "new-key", Auth: "new-auth"},
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("upsert: expected 204, got %d", w.Code)
	}

	// Verify via storage — should have exactly 1 subscription
	subs, err := store.ListPushSubscriptions(t.Context(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription after upsert, got %d", len(subs))
	}
	if subs[0].P256DH != "new-key" {
		t.Errorf("p256dh = %q, want new-key", subs[0].P256DH)
	}
}

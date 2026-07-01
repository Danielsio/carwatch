package yad2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRelayClient_RewritesURL(t *testing.T) {
	var gotTarget, gotSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTarget = r.URL.Query().Get("target")
		gotSecret = r.Header.Get("X-Relay-Secret")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewRelayClient(srv.URL, "test-secret", []string{"TestAgent/1.0"})
	targetURL := "https://gw.yad2.co.il/feed-search-legacy/vehicles/cars?manufacturer=17&model=10188"

	result, err := c.Get(context.Background(), targetURL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("status = %d, want 200", result.StatusCode)
	}
	if gotTarget != targetURL {
		t.Errorf("target = %q, want %q", gotTarget, targetURL)
	}
	if gotSecret != "test-secret" {
		t.Errorf("secret = %q, want %q", gotSecret, "test-secret")
	}
}

func TestRelayClient_ForwardsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test"}`))
	}))
	defer srv.Close()

	c := NewRelayClient(srv.URL, "s", []string{"TestAgent/1.0"})
	result, err := c.Get(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(result.Body) != `{"data":"test"}` {
		t.Errorf("body = %q, want %q", result.Body, `{"data":"test"}`)
	}
	if result.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content-type = %q, want application/json", result.Header.Get("Content-Type"))
	}
}

func TestRelayClient_WorkerAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Relay-Secret") != "correct" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewRelayClient(srv.URL, "wrong", []string{"TestAgent/1.0"})
	result, err := c.Get(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.StatusCode != 401 {
		t.Errorf("status = %d, want 401", result.StatusCode)
	}
}

func TestRelayClient_WorkerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("relay fetch failed: connection refused"))
	}))
	defer srv.Close()

	c := NewRelayClient(srv.URL, "s", []string{"TestAgent/1.0"})
	result, err := c.Get(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.StatusCode != 502 {
		t.Errorf("status = %d, want 502", result.StatusCode)
	}
}

func TestRelayClient_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewRelayClient(srv.URL, "s", []string{"TestAgent/1.0"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Get(ctx, "https://example.com")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRelayClient_ComplexURLEncoding(t *testing.T) {
	var gotTarget string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTarget = r.URL.Query().Get("target")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewRelayClient(srv.URL, "s", []string{"TestAgent/1.0"})
	targetURL := "https://gw.yad2.co.il/feed-search-legacy/vehicles/cars?manufacturer=19&model=10226&year=2019-2024&price=0-95000&km=-1-70000&hand=0-2&ownerID=1&imgOnly=1&Order=1&page=1"

	_, err := c.Get(context.Background(), targetURL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	decoded, _ := url.QueryUnescape(gotTarget)
	if decoded != targetURL {
		t.Errorf("decoded target = %q, want %q", decoded, targetURL)
	}
}

func TestRelayClient_ForwardsBrowserHeaders(t *testing.T) {
	var gotUA, gotLang, gotDNT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotLang = r.Header.Get("Accept-Language")
		gotDNT = r.Header.Get("DNT")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewRelayClient(srv.URL, "s", []string{"Mozilla/5.0 Test"})
	_, err := c.Get(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotUA != "Mozilla/5.0 Test" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "Mozilla/5.0 Test")
	}
	if gotLang == "" {
		t.Error("Accept-Language not forwarded")
	}
	if gotDNT != "1" {
		t.Errorf("DNT = %q, want %q", gotDNT, "1")
	}
}

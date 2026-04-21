package winwin

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestNewClient(t *testing.T) {
	c, err := NewClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_WithProxy(t *testing.T) {
	c, err := NewClient([]string{"TestAgent/1.0"}, "http://proxy.example.com:8080")
	if err != nil {
		t.Fatalf("NewClient with proxy: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_InvalidProxy(t *testing.T) {
	_, err := NewClient([]string{"TestAgent/1.0"}, "://invalid")
	if err == nil {
		t.Error("expected error for invalid proxy URL")
	}
}

func TestClient_Do_SetsHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c, _ := NewClient([]string{"TestAgent/1.0"}, "")
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if receivedHeaders.Get("User-Agent") != "TestAgent/1.0" {
		t.Errorf("User-Agent = %q, want TestAgent/1.0", receivedHeaders.Get("User-Agent"))
	}
	if receivedHeaders.Get("Accept-Language") == "" {
		t.Error("Accept-Language header not set")
	}
	if receivedHeaders.Get("DNT") != "1" {
		t.Error("DNT header not set")
	}
}

func TestNewFetcherWithProxyPool(t *testing.T) {
	pool := fetcher.NewProxyPool([]string{"http://proxy1.example.com:8080"})
	f, err := NewFetcherWithProxyPool([]string{"TestAgent/1.0"}, pool, discardLogger)
	if err != nil {
		t.Fatalf("NewFetcherWithProxyPool: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil fetcher")
	}
	if f.proxyPool != pool {
		t.Error("proxy pool not set")
	}
}

func TestNewFetcherWithProxyPool_NilPool(t *testing.T) {
	f, err := NewFetcherWithProxyPool([]string{"TestAgent/1.0"}, nil, discardLogger)
	if err != nil {
		t.Fatalf("NewFetcherWithProxyPool nil: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil fetcher")
	}
}

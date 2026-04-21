package yad2

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/config"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func validPageHTML() string {
	return `<!DOCTYPE html><html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{"data":{"feed":{"feed_items":[
{"token":"tok-1","manufacturer":{"text":"Mazda","english_text":"Mazda","id":27},"model":{"text":"3","english_text":"3","id":10332},"year_of_production":2021,"price":95000}
]}}}}}]}}}}
</script></body></html>`
}

func TestNewFetcher(t *testing.T) {
	f, err := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil fetcher")
	}
	if f.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", f.baseURL, defaultBaseURL)
	}
}

func TestNewFetcher_WithProxy(t *testing.T) {
	f, err := NewFetcher([]string{"TestAgent/1.0"}, "http://proxy.example.com:8080", discardLogger)
	if err != nil {
		t.Fatalf("NewFetcher with proxy: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil fetcher")
	}
}

func TestNewFetcher_InvalidProxy(t *testing.T) {
	_, err := NewFetcher([]string{"TestAgent/1.0"}, "://invalid", discardLogger)
	if err == nil {
		t.Error("expected error for invalid proxy URL")
	}
}

func TestNewFetcherWithProxyPool(t *testing.T) {
	f, err := NewFetcherWithProxyPool([]string{"TestAgent/1.0"}, nil, discardLogger)
	if err != nil {
		t.Fatalf("NewFetcherWithProxyPool: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil fetcher")
	}
}

func TestYad2Fetcher_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(validPageHTML()))
	}))
	defer server.Close()

	f, _ := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	f.baseURL = server.URL

	listings, err := f.Fetch(context.Background(), config.SourceParams{Manufacturer: 27, Model: 10332})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].Token != "tok-1" {
		t.Errorf("token = %q, want tok-1", listings[0].Token)
	}
}

func TestYad2Fetcher_Fetch_GzipResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte(validPageHTML()))
		gz.Close()
	}))
	defer server.Close()

	f, _ := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	f.baseURL = server.URL

	listings, err := f.Fetch(context.Background(), config.SourceParams{Manufacturer: 27, Model: 10332})
	if err != nil {
		t.Fatalf("Fetch gzip: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
}

func TestYad2Fetcher_Fetch_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	f, _ := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	f.baseURL = server.URL

	_, err := f.Fetch(context.Background(), config.SourceParams{})
	if err == nil {
		t.Error("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestYad2Fetcher_Fetch_ServerDown(t *testing.T) {
	f, _ := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	f.baseURL = "http://127.0.0.1:1"

	_, err := f.Fetch(context.Background(), config.SourceParams{})
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestYad2Fetcher_Fetch_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(validPageHTML()))
	}))
	defer server.Close()

	f, _ := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	f.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.Fetch(ctx, config.SourceParams{})
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestYad2Fetcher_Fetch_Challenge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>Are you for real?</body></html>`))
	}))
	defer server.Close()

	f, _ := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	f.baseURL = server.URL

	_, err := f.Fetch(context.Background(), config.SourceParams{})
	if err == nil {
		t.Error("expected error for challenge page")
	}
}

func TestParseListingsPage_InlineHTML(t *testing.T) {
	html := validPageHTML()
	listings, err := ParseListingsPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseListingsPage: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0].ManufacturerID != 27 || listings[0].Manufacturer != "Mazda" {
		t.Errorf("listing = %+v", listings[0])
	}
	if listings[0].ModelID != 10332 || listings[0].Model != "3" {
		t.Errorf("model = %+v", listings[0])
	}
}

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

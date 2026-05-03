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

	"github.com/dsionov/carwatch/internal/model"
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

func newTestFetcher(t *testing.T, serverURL string) *Yad2Fetcher {
	t.Helper()
	client, err := NewPlainClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatalf("NewPlainClient: %v", err)
	}
	ic, err := NewPlainClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatalf("NewPlainClient (itemClient): %v", err)
	}
	return &Yad2Fetcher{client: client, itemClient: ic, baseURL: serverURL, logger: discardLogger, userAgents: []string{"TestAgent/1.0"}}
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

	f := newTestFetcher(t, server.URL)
	listings, err := f.Fetch(context.Background(), defaultParams())
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
		defer func() { _ = gz.Close() }()
		_, _ = gz.Write([]byte(validPageHTML()))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	listings, err := f.Fetch(context.Background(), defaultParams())
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

	f := newTestFetcher(t, server.URL)
	_, err := f.Fetch(context.Background(), defaultParams())
	if err == nil {
		t.Error("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestYad2Fetcher_Fetch_ServerDown(t *testing.T) {
	client, err := NewPlainClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatalf("NewPlainClient: %v", err)
	}
	ic, err := NewPlainClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatalf("NewPlainClient (itemClient): %v", err)
	}
	f := &Yad2Fetcher{client: client, itemClient: ic, baseURL: "http://127.0.0.1:1", logger: discardLogger, userAgents: []string{"TestAgent/1.0"}}

	_, err = f.Fetch(context.Background(), defaultParams())
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestYad2Fetcher_Fetch_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(validPageHTML()))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.Fetch(ctx, defaultParams())
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestYad2Fetcher_Fetch_Challenge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>Are you for real?</body></html>`))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	_, err := f.Fetch(context.Background(), defaultParams())
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
	c.Close()
}

func TestNewClient_WithProxy(t *testing.T) {
	c, err := NewClient([]string{"TestAgent/1.0"}, "http://proxy.example.com:8080")
	if err != nil {
		t.Fatalf("NewClient with proxy: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	c.Close()
}

func TestNewClient_InvalidProxy(t *testing.T) {
	_, err := NewClient([]string{"TestAgent/1.0"}, "://invalid")
	if err == nil {
		t.Error("expected error for invalid proxy URL")
	}
}

func TestNewClient_EmptyUserAgents(t *testing.T) {
	_, err := NewClient(nil, "")
	if err == nil {
		t.Error("expected error for empty user agents")
	}
}

func TestPlainClient_Get_SetsHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c, cErr := NewPlainClient([]string{"TestAgent/1.0"}, "")
	if cErr != nil {
		t.Fatalf("NewPlainClient: %v", cErr)
	}
	result, err := c.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("status = %d, want 200", result.StatusCode)
	}

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

func TestYad2Fetcher_FetchItem_UsesItemClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/vehicles/item/") {
			_, _ = w.Write([]byte(`<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":55000,"address":{"city":{"text":"חיפה","textEng":"haifa"}}}}}}
</script></html>`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(validPageHTML()))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	details, err := f.FetchItem(context.Background(), "tok-1")
	if err != nil {
		t.Fatalf("FetchItem: %v", err)
	}
	if details.Km != 55000 {
		t.Errorf("Km = %d, want 55000", details.Km)
	}
}

func TestYad2Fetcher_FetchItem_ParsesCityAndKm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"itemData":{"km":30000,"address":{"city":{"text":"באר שבע","textEng":"beer_sheva"}}}}}}
</script></html>`))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	details, err := f.FetchItem(context.Background(), "tok-2")
	if err != nil {
		t.Fatalf("FetchItem: %v", err)
	}
	if details.Km != 30000 {
		t.Errorf("Km = %d, want 30000", details.Km)
	}
	if details.City != "beer_sheva" {
		t.Errorf("City = %q, want beer_sheva", details.City)
	}
}

func TestYad2Fetcher_FetchItem_BotProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html>validate.perfdrive.com challenge</html>`))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	_, err := f.FetchItem(context.Background(), "tok-blocked")
	if err == nil {
		t.Fatal("expected error for bot protection response")
	}
	if !strings.Contains(err.Error(), "anti-bot challenge") {
		t.Errorf("error should mention anti-bot challenge: %v", err)
	}
}

func TestYad2Fetcher_Fetch_BotProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html>ShieldSquare captcha page</html>`))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	_, err := f.Fetch(context.Background(), defaultParams())
	if err == nil {
		t.Fatal("expected error for bot protection response")
	}
	if !strings.Contains(err.Error(), "anti-bot challenge") {
		t.Errorf("error should mention anti-bot challenge: %v", err)
	}
}

func TestLooksLikeBotProtection(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"perfdrive", "<html>validate.perfdrive.com</html>", true},
		{"shieldsquare", "<html>ShieldSquare captcha</html>", true},
		{"are you for real", "<html>Are you for real?</html>", true},
		{"cf-browser", "<html>cf-browser-verification</html>", true},
		{"captcha", "<html>Please complete the CAPTCHA</html>", true},
		{"normal page", "<html><body>Normal content</body></html>", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeBotProtection([]byte(tc.body)); got != tc.want {
				t.Errorf("looksLikeBotProtection(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestYad2Fetcher_FetchItem_DoesNotPoisonListingClient(t *testing.T) {
	listingHandler := func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("poison"); err == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`<html><head><title>400 Bad Request</title></head></html>`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(validPageHTML()))
	}
	itemHandler := func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "poison", Value: "bad"})
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<html><head><title>400 Bad Request</title></head></html>`))
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/vehicles/item/") {
			itemHandler(w, r)
			return
		}
		listingHandler(w, r)
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)

	listings, err := f.Fetch(context.Background(), defaultParams())
	if err != nil {
		t.Fatalf("Fetch before item: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}

	_, err = f.FetchItem(context.Background(), "tok-1")
	if err == nil {
		t.Fatal("expected error for 400 item response")
	}

	listings, err = f.Fetch(context.Background(), defaultParams())
	if err != nil {
		t.Fatalf("Fetch after poisoned item should still succeed: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing after item failure, got %d", len(listings))
	}
}

func TestYad2Fetcher_FetchItem_Generic400IsChallenge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<html><head><title>400 Bad Request</title></head><body><center><h1>400 Bad Request</h1></center><hr><center>nginx</center></body></html>`))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	_, err := f.FetchItem(context.Background(), "tok-generic400")
	if err == nil {
		t.Fatal("expected error for generic 400 response")
	}
	if !strings.Contains(err.Error(), "anti-bot challenge") {
		t.Errorf("error should mention anti-bot challenge: %v", err)
	}
}

func TestYad2Fetcher_FetchItem_Generic403IsChallenge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><head><title>403 Forbidden</title></head><body><center><h1>403 Forbidden</h1></center><hr><center>nginx</center></body></html>`))
	}))
	defer server.Close()

	f := newTestFetcher(t, server.URL)
	_, err := f.FetchItem(context.Background(), "tok-generic403")
	if err == nil {
		t.Fatal("expected error for generic 403 response")
	}
	if !strings.Contains(err.Error(), "anti-bot challenge") {
		t.Errorf("error should mention anti-bot challenge: %v", err)
	}
}

func TestLooksLikeGenericError(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"400 title", "<html><head><title>400 Bad Request</title></head></html>", true},
		{"403 title", "<html><head><title>403 Forbidden</title></head></html>", true},
		{"normal page", "<html><body>Normal content</body></html>", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeGenericError([]byte(tc.body)); got != tc.want {
				t.Errorf("looksLikeGenericError(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestYad2Fetcher_Close_Idempotent(t *testing.T) {
	f, err := NewFetcher([]string{"TestAgent/1.0"}, "", discardLogger)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	f.Close()
	f.Close()
}

func defaultParams() model.SourceParams {
	return model.SourceParams{Manufacturer: 27, Model: 10332}
}

package winwin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/model"
)

func TestWinWinFetcher_FetchParsesHTML(t *testing.T) {
	html := `<!DOCTYPE html><html><body>
<div><a href="/vehicles/private/9000123">מאזדה 3 2021</a>
<span>120,000 ₪</span><span>50,000 קמ</span><span>יד 2</span><span>• תל אביב</span></div>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	t.Cleanup(srv.Close)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	f := &WinWinFetcher{
		userAgents: []string{"TestAgent/1.0"},
		baseURL:    srv.URL,
		logger:     logger,
	}
	var err error
	f.client, err = NewClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatal(err)
	}

	listings, err := f.Fetch(context.Background(), model.SourceParams{Manufacturer: 27, Model: 10332})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("want 1 listing, got %d", len(listings))
	}
	l := listings[0]
	if l.Token != "9000123" || l.Price != 120000 || l.Km != 50000 || l.Hand != 2 || l.Year != 2021 {
		t.Fatalf("listing: %+v", l)
	}
}

func TestWinWinFetcher_ChallengeStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`errors.edgesuite.net`))
	}))
	t.Cleanup(srv.Close)
	f := &WinWinFetcher{
		userAgents: []string{"TestAgent/1.0"},
		baseURL:    srv.URL,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	var err error
	f.client, err = NewClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Fetch(context.Background(), model.SourceParams{})
	if !errors.Is(err, fetcher.ErrChallenge) {
		t.Fatalf("want ErrChallenge, got %v", err)
	}
}

func TestWinWinFetcher_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	f := &WinWinFetcher{
		userAgents: []string{"TestAgent/1.0"},
		baseURL:    srv.URL,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	var err error
	f.client, err = NewClient([]string{"TestAgent/1.0"}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Fetch(context.Background(), model.SourceParams{})
	if !errors.Is(err, fetcher.ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name   string
		params model.SourceParams
		want   string
	}{
		{name: "empty", params: model.SourceParams{}, want: "https://www.winwin.co.il/vehicles/cars-for-sale"},
		{
			name: "full",
			params: model.SourceParams{
				Manufacturer: 27, Model: 10332, YearMin: 2020, YearMax: 2024, PriceMax: 150000,
				MaxKm: 120000, MaxHand: 3, EngineMinCC: 1600,
			},
			want: "https://www.winwin.co.il/vehicles/cars-for-sale?engineVolume=1600&hand=3&km=120000&manufacturer=27&model=10332&priceTo=150000&yearFrom=2020&yearTo=2024",
		},
		{name: "page", params: model.SourceParams{Manufacturer: 35, Page: 2},
			want: "https://www.winwin.co.il/vehicles/cars-for-sale?manufacturer=35&page=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildURL(defaultBaseURL, tt.params); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestParseListingsPage_ChallengeHTML(t *testing.T) {
	_, err := ParseListingsPage(strings.NewReader(`<html>errors.edgesuite.net</html>`))
	if !errors.Is(err, fetcher.ErrChallenge) {
		t.Fatal(err)
	}
}

func TestParseListingsPage_EmptyDoc(t *testing.T) {
	listings, err := ParseListingsPage(strings.NewReader("<html></html>"))
	if err != nil || len(listings) != 0 {
		t.Fatalf("err=%v n=%d", err, len(listings))
	}
}

func TestParseListingsPage_NilReader(t *testing.T) {
	listings, err := ParseListingsPage(nil)
	if err != nil || listings != nil {
		t.Fatalf("err=%v listings=%v", err, listings)
	}
}

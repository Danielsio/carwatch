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

func TestParseListingsPage_Fixture(t *testing.T) {
	f, err := os.Open("../../../testdata/winwin_page.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	listings, err := ParseListingsPage(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 3 {
		t.Fatalf("want 3 listings (4th is duplicate), got %d", len(listings))
	}

	// Listing 1: Toyota Corolla
	l := listings[0]
	if l.Token != "12345" {
		t.Errorf("listing[0].Token = %q, want 12345", l.Token)
	}
	if l.Year != 2022 {
		t.Errorf("listing[0].Year = %d, want 2022", l.Year)
	}
	if l.Price != 149000 {
		t.Errorf("listing[0].Price = %d, want 149000", l.Price)
	}
	if l.Km != 45000 {
		t.Errorf("listing[0].Km = %d, want 45000", l.Km)
	}
	if l.Hand != 2 {
		t.Errorf("listing[0].Hand = %d, want 2", l.Hand)
	}

	// Listing 2: Mazda 3
	l = listings[1]
	if l.Token != "67890" {
		t.Errorf("listing[1].Token = %q, want 67890", l.Token)
	}
	if l.Year != 2021 {
		t.Errorf("listing[1].Year = %d, want 2021", l.Year)
	}
	if l.Price != 125000 {
		t.Errorf("listing[1].Price = %d, want 125000", l.Price)
	}
	if l.Km != 60000 {
		t.Errorf("listing[1].Km = %d, want 60000", l.Km)
	}
	if l.Hand != 1 {
		t.Errorf("listing[1].Hand = %d, want 1", l.Hand)
	}

	// Listing 3: Hyundai Tucson
	l = listings[2]
	if l.Token != "11223" {
		t.Errorf("listing[2].Token = %q, want 11223", l.Token)
	}
	if l.Year != 2023 {
		t.Errorf("listing[2].Year = %d, want 2023", l.Year)
	}
	if l.Price != 189000 {
		t.Errorf("listing[2].Price = %d, want 189000", l.Price)
	}
	if l.Km != 20000 {
		t.Errorf("listing[2].Km = %d, want 20000", l.Km)
	}
	if l.Hand != 1 {
		t.Errorf("listing[2].Hand = %d, want 1", l.Hand)
	}

	// Verify page links are fully resolved
	for i, ll := range listings {
		if !strings.HasPrefix(ll.PageLink, "https://www.winwin.co.il/") {
			t.Errorf("listing[%d].PageLink = %q, want https://www.winwin.co.il/ prefix", i, ll.PageLink)
		}
	}
}

func TestParseListingsPage_CardClassScope(t *testing.T) {
	// Verify that the parser uses class-based card detection when available.
	// The anchor is deeply nested (>6 levels), but the card class is at level 3.
	html := `<!DOCTYPE html><html><body>
<div class="page">
  <div class="results">
    <div class="listing-card">
      <div class="inner">
        <div class="wrap">
          <div class="deep1">
            <div class="deep2">
              <div class="deep3">
                <a href="/vehicles/private/99999">סובארו XV 2020</a>
              </div>
            </div>
          </div>
        </div>
        <span>95,000 ₪</span>
        <span>80,000 קמ</span>
      </div>
    </div>
  </div>
</div>
</body></html>`

	listings, err := ParseListingsPage(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("want 1 listing, got %d", len(listings))
	}
	l := listings[0]
	if l.Price != 95000 {
		t.Errorf("Price = %d, want 95000 (card scope should include price)", l.Price)
	}
	if l.Km != 80000 {
		t.Errorf("Km = %d, want 80000 (card scope should include km)", l.Km)
	}
}

func TestExtractCityHeuristic_Separators(t *testing.T) {
	tests := []struct {
		name string
		blob string
		want string
	}{
		{name: "bullet", blob: "some text • תל אביב", want: "תל אביב"},
		{name: "pipe", blob: "some text | חיפה", want: "חיפה"},
		{name: "middle_dot", blob: "some text · ירושלים", want: "ירושלים"},
		{name: "dash", blob: "some text - באר שבע", want: "באר שבע"},
		{name: "comma", blob: "some text, נתניה", want: "נתניה"},
		{name: "no_separator", blob: "some text without city", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCityHeuristic(tt.blob)
			if got != tt.want {
				t.Errorf("extractCityHeuristic(%q) = %q, want %q", tt.blob, got, tt.want)
			}
		})
	}
}

func TestFetch_ZeroResultsWarning(t *testing.T) {
	// Return a large HTML body with no vehicle links — should log a warning but not error.
	bigBody := "<html><body>" + strings.Repeat("<p>filler content</p>", 200) + "</body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(bigBody))
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

	listings, err := f.Fetch(context.Background(), model.SourceParams{})
	if err != nil {
		t.Fatalf("Fetch should not error on zero results: %v", err)
	}
	if len(listings) != 0 {
		t.Fatalf("expected 0 listings, got %d", len(listings))
	}
}

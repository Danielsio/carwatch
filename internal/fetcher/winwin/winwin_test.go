package winwin

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/dsionov/carwatch/internal/config"
)

func TestWinWinFetcher_StubReturnsEmpty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	f, err := NewFetcher([]string{"TestAgent/1.0"}, "", logger)
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}

	listings, err := f.Fetch(context.Background(), config.SourceParams{
		Manufacturer: 27,
		Model:        10332,
		YearMin:      2020,
		YearMax:      2024,
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(listings) != 0 {
		t.Errorf("expected 0 listings from stub, got %d", len(listings))
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name   string
		params config.SourceParams
		want   string
	}{
		{
			name:   "empty params",
			params: config.SourceParams{},
			want:   "https://www.winwin.co.il/vehicles/cars",
		},
		{
			name: "full params",
			params: config.SourceParams{
				Manufacturer: 27,
				Model:        10332,
				YearMin:      2020,
				YearMax:      2024,
				PriceMax:     150000,
			},
			want: "https://www.winwin.co.il/vehicles/cars?manufacturer=27&model=10332&priceTo=150000&yearFrom=2020&yearTo=2024",
		},
		{
			name: "with page",
			params: config.SourceParams{
				Manufacturer: 35,
				Page:         2,
			},
			want: "https://www.winwin.co.il/vehicles/cars?manufacturer=35&page=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildURL(defaultBaseURL, tt.params)
			if got != tt.want {
				t.Errorf("buildURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseListingsPage_Stub(t *testing.T) {
	listings, err := ParseListingsPage(nil)
	if err != nil {
		t.Fatalf("ParseListingsPage: %v", err)
	}
	if listings != nil {
		t.Errorf("expected nil from stub parser, got %v", listings)
	}
}

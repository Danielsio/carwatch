//go:build e2e

package e2e

import (
	"os"
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/scoring"
)

// TestIntegration_ParserToFilterToScorerToFormatter verifies the full data
// pipeline from HTML parsing through filtering, scoring, and notification
// formatting — using real Yad2 HTML fixtures and real component code,
// not mocks.
func TestIntegration_ParserToFilterToScorerToFormatter(t *testing.T) {
	html, err := os.Open("../testdata/yad2_page.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	defer func() { _ = html.Close() }()

	listings, err := yad2.ParseListingsPage(html)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(listings) == 0 {
		t.Fatal("expected parsed listings from fixture, got 0")
	}

	t.Logf("parsed %d listings from Yad2 fixture", len(listings))

	for i, l := range listings {
		if l.Token == "" {
			t.Errorf("listing[%d] has empty token", i)
		}
	}

	// Apply filter criteria typical for a user search
	criteria := model.FilterCriteria{
		YearMin:  2018,
		YearMax:  2025,
		PriceMax: 200000,
		MaxHand:  3,
	}
	filtered := filter.Apply(criteria, listings)
	t.Logf("filtered %d → %d listings", len(listings), len(filtered))

	if len(filtered) > len(listings) {
		t.Error("filter should not increase listing count")
	}

	for _, l := range filtered {
		if l.Year > 0 && (l.Year < criteria.YearMin || l.Year > criteria.YearMax) {
			t.Errorf("listing %s has year %d outside filter range [%d, %d]",
				l.Token, l.Year, criteria.YearMin, criteria.YearMax)
		}
		if l.Price > criteria.PriceMax {
			t.Errorf("listing %s has price %d above PriceMax %d",
				l.Token, l.Price, criteria.PriceMax)
		}
		if l.Hand > criteria.MaxHand {
			t.Errorf("listing %s has hand %d above MaxHand %d",
				l.Token, l.Hand, criteria.MaxHand)
		}
	}

	// Score each filtered listing
	var scoredListings []model.Listing
	for _, raw := range filtered {
		listing := model.Listing{RawListing: raw}

		if raw.Price > 0 {
			listing.FitnessScore = scoring.FitnessScore(scoring.FitnessParams{
				Price:    raw.Price,
				Km:       raw.Km,
				Hand:     raw.Hand,
				Year:     raw.Year,
				YearMin:  criteria.YearMin,
				YearMax:  criteria.YearMax,
				PriceMax: criteria.PriceMax,
			})
		}

		scoredListings = append(scoredListings, listing)
	}

	for _, l := range scoredListings {
		if l.FitnessScore < 0 || l.FitnessScore > 10 {
			t.Errorf("listing %s has fitness score %.1f outside [0,10]",
				l.Token, l.FitnessScore)
		}
	}

	// Format for Telegram notification
	if len(scoredListings) > 0 {
		msg := notifier.FormatBatch(scoredListings, locale.Hebrew)
		if msg == "" {
			t.Error("formatted message is empty")
		}
		if !strings.Contains(msg, "🔗") {
			t.Error("formatted message should contain link emoji")
		}
		t.Logf("formatted %d listings into %d-char message", len(scoredListings), len(msg))

		// Verify individual listings format without error
		for i, l := range scoredListings {
			single := notifier.FormatListing(l, locale.Hebrew)
			if single == "" {
				t.Errorf("FormatListing[%d] returned empty string", i)
			}
		}
	}
}

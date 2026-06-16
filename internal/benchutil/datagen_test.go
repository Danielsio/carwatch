package benchutil

import (
	"math/rand/v2" //nolint:gosec // deterministic seed for reproducible test data
	"testing"
)

func TestGenerateUsers(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
	users := GenerateUsers(rng, 10, 3)

	if len(users) != 10 {
		t.Fatalf("got %d users, want 10", len(users))
	}
	for i, u := range users {
		if len(u.Searches) != 3 {
			t.Errorf("user %d: got %d searches, want 3", i, len(u.Searches))
		}
		for j, s := range u.Searches {
			if s.ChatID != u.ChatID {
				t.Errorf("user %d search %d: ChatID mismatch", i, j)
			}
			if s.Manufacturer == 0 || s.Model == 0 {
				t.Errorf("user %d search %d: zero manufacturer/model", i, j)
			}
			if s.YearMin > s.YearMax {
				t.Errorf("user %d search %d: YearMin %d > YearMax %d", i, j, s.YearMin, s.YearMax)
			}
			if s.PriceMin > s.PriceMax {
				t.Errorf("user %d search %d: PriceMin %d > PriceMax %d", i, j, s.PriceMin, s.PriceMax)
			}
		}
	}
}

func TestGenerateListings(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
	listings := GenerateListings(rng, 200)

	if len(listings) != 200 {
		t.Fatalf("got %d listings, want 200", len(listings))
	}

	tokens := make(map[string]bool, len(listings))
	for _, l := range listings {
		if tokens[l.Token] {
			t.Errorf("duplicate token: %s", l.Token)
		}
		tokens[l.Token] = true

		if l.ManufacturerID == 0 {
			t.Error("zero ManufacturerID")
		}
		if l.Price <= 0 {
			t.Error("non-positive Price")
		}
	}
}

func TestAllSearches(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
	users := GenerateUsers(rng, 5, 4)
	all := AllSearches(users)
	if len(all) != 20 {
		t.Errorf("got %d searches, want 20", len(all))
	}
}

func TestToListingRecords(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
	listings := GenerateListings(rng, 3)
	records := ToListingRecords(listings, 1234, 55, "bench-search")
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	for i := range records {
		if records[i].ChatID != 1234 || records[i].SearchID != 55 || records[i].SearchName != "bench-search" {
			t.Fatalf("record %d: search metadata mismatch", i)
		}
		if records[i].Token != listings[i].Token {
			t.Fatalf("record %d: token mismatch", i)
		}
		if records[i].FitnessScore == nil {
			t.Fatalf("record %d: nil FitnessScore", i)
		}
	}
}

func TestGenerateMedianEntries(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
	entries := GenerateMedianEntries(rng, 20)
	if len(entries) != 20 {
		t.Fatalf("got %d entries, want 20", len(entries))
	}
	for i, e := range entries {
		if e.Manufacturer == "" || e.Model == "" {
			t.Fatalf("entry %d: missing manufacturer/model", i)
		}
		if e.MedianPrice <= 0 || e.MedianKm <= 0 || e.CohortSize <= 0 {
			t.Fatalf("entry %d: non-positive numeric field", i)
		}
	}
}

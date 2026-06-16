package percolator

import (
	"fmt"
	"math/rand/v2" //nolint:gosec // deterministic seed for reproducible benchmarks
	"testing"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func generateSearches(rng *rand.Rand, n int) []storage.Search {
	manufacturers := []int{27, 19, 21, 7, 22, 10, 3, 24, 14, 5}
	models := []int{10332, 10333, 1, 2, 5001, 6001, 7001, 8001, 9001, 9501}

	searches := make([]storage.Search, n)
	for i := range n {
		yearMin := 2016 + rng.IntN(6)
		priceMin := 40_000 + rng.IntN(40_000)
		searches[i] = storage.Search{
			ID:           int64(i + 1),
			ChatID:       int64(100_000 + i),
			Manufacturer: manufacturers[rng.IntN(len(manufacturers))],
			Model:        models[rng.IntN(len(models))],
			YearMin:      yearMin,
			YearMax:      yearMin + 2 + rng.IntN(4),
			PriceMin:     priceMin,
			PriceMax:     priceMin + 30_000 + rng.IntN(50_000),
			Active:       true,
		}
	}
	return searches
}

func generateListings(rng *rand.Rand, n int) []model.RawListing {
	manufacturers := []struct {
		id   int
		name string
	}{
		{27, "Mazda"}, {19, "Toyota"}, {21, "Honda"},
		{7, "Hyundai"}, {22, "Kia"}, {10, "Nissan"},
	}
	models := []struct {
		id   int
		name string
	}{
		{10332, "3"}, {1, "Corolla"}, {5001, "Civic"},
		{6001, "i30"}, {7001, "Sportage"}, {8001, "Qashqai"},
	}

	listings := make([]model.RawListing, n)
	for i := range n {
		mfr := manufacturers[rng.IntN(len(manufacturers))]
		mdl := models[rng.IntN(len(models))]
		listings[i] = model.RawListing{
			Token:          fmt.Sprintf("tok-%d", i),
			ManufacturerID: mfr.id,
			Manufacturer:   mfr.name,
			ModelID:        mdl.id,
			Model:          mdl.name,
			Year:           2016 + rng.IntN(10),
			Price:          30_000 + rng.IntN(150_000),
			Km:             rng.IntN(250_000),
			Hand:           1 + rng.IntN(4),
			EngineVolume:   1400 + float64(rng.IntN(1200)),
		}
	}
	return listings
}

func BenchmarkMatch(b *testing.B) {
	for _, searchCount := range []int{50, 100, 350, 1000} {
		b.Run(fmt.Sprintf("searches=%d", searchCount), func(b *testing.B) {
			rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
			searches := generateSearches(rng, searchCount)
			listing := generateListings(rng, 1)[0]

			p := New()
			p.Load(searches)

			b.ResetTimer()
			for range b.N {
				p.Match(listing)
			}
		})
	}
}

func BenchmarkMatchBatch(b *testing.B) {
	rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
	searches := generateSearches(rng, 350)
	listings := generateListings(rng, 200)

	p := New()
	p.Load(searches)

	b.ResetTimer()
	for range b.N {
		for _, l := range listings {
			p.Match(l)
		}
	}
}

func BenchmarkCountRejections(b *testing.B) {
	rng := rand.New(rand.NewPCG(42, 0)) //nolint:gosec
	searches := generateSearches(rng, 350)
	listings := generateListings(rng, 200)

	p := New()
	p.Load(searches)

	b.ResetTimer()
	for range b.N {
		p.CountRejections(listings)
	}
}

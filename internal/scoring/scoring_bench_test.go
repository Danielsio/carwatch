package scoring

import (
	"math/rand/v2" //nolint:gosec // deterministic seed for reproducible benchmarks
	"testing"
)

func BenchmarkFitnessScoreDetailed(b *testing.B) {
	rng := rand.New(rand.NewPCG(42, 0))
	params := make([]FitnessParams, 200)
	for i := range params {
		params[i] = FitnessParams{
			Price:        30_000 + rng.IntN(150_000),
			Km:           rng.IntN(250_000),
			Hand:         1 + rng.IntN(4),
			Year:         2016 + rng.IntN(10),
			EngineVolume: 1400 + float64(rng.IntN(1200)),
			PriceMax:     200_000,
			MaxKm:        200_000,
			MaxHand:      4,
			YearMin:      2016,
			YearMax:      2026,
			EngineMinCC:  1400,
			MedianPrice:  50_000 + rng.IntN(100_000),
			MedianKm:     30_000 + rng.IntN(150_000),
		}
	}

	b.ResetTimer()
	for range b.N {
		for i := range params {
			FitnessScoreDetailed(params[i])
		}
	}
}

func BenchmarkScoreWithKm(b *testing.B) {
	rng := rand.New(rand.NewPCG(42, 0))
	type scoreInput struct {
		price, km, medianPrice, medianKm int
	}
	inputs := make([]scoreInput, 200)
	for i := range inputs {
		inputs[i] = scoreInput{
			price:       30_000 + rng.IntN(150_000),
			km:          rng.IntN(250_000),
			medianPrice: 50_000 + rng.IntN(100_000),
			medianKm:    30_000 + rng.IntN(150_000),
		}
	}

	b.ResetTimer()
	for range b.N {
		for _, in := range inputs {
			ScoreWithKm(in.price, in.km, in.medianPrice, in.medianKm)
		}
	}
}

func BenchmarkMarketCacheLookup(b *testing.B) {
	rng := rand.New(rand.NewPCG(42, 0))
	manufacturers := []string{"Mazda", "Toyota", "Honda", "Hyundai", "Kia"}
	models := []string{"3", "Corolla", "Civic", "i30", "Sportage"}

	entries := make([]MedianEntry, 500)
	for i := range entries {
		entries[i] = MedianEntry{
			Manufacturer: manufacturers[rng.IntN(len(manufacturers))],
			Model:        models[rng.IntN(len(models))],
			Year:         2016 + rng.IntN(10),
			MedianPrice:  50_000 + rng.IntN(100_000),
			MedianKm:     30_000 + rng.IntN(150_000),
			CohortSize:   10 + rng.IntN(90),
		}
	}
	mc := NewMarketCacheFromMedians(entries)

	b.ResetTimer()
	for range b.N {
		for _, e := range entries {
			mc.Lookup(e.Manufacturer, e.Model, e.Year)
		}
	}
}

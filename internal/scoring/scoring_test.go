package scoring

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
)

func TestScore_BelowMedian(t *testing.T) {
	s := Score(80000, 100000)
	if s != 20 {
		t.Errorf("expected 20, got %d", s)
	}
}

func TestScore_AtMedian(t *testing.T) {
	s := Score(100000, 100000)
	if s != 0 {
		t.Errorf("expected 0, got %d", s)
	}
}

func TestScore_AboveMedian(t *testing.T) {
	s := Score(120000, 100000)
	if s != 0 {
		t.Errorf("expected 0 (clamped), got %d", s)
	}
}

func TestScore_VeryLow(t *testing.T) {
	s := Score(1000, 100000)
	if s != 99 {
		t.Errorf("expected 99, got %d", s)
	}
}

func TestScore_ZeroMedian(t *testing.T) {
	s := Score(80000, 0)
	if s != 0 {
		t.Errorf("expected 0 for zero median, got %d", s)
	}
}

func TestScore_ZeroPrice(t *testing.T) {
	s := Score(0, 100000)
	if s != 0 {
		t.Errorf("expected 0 for zero price, got %d", s)
	}
}

func TestMarketCache_LookupSufficient(t *testing.T) {
	var data []ListingData
	for i := range 15 {
		data = append(data, ListingData{
			Manufacturer: "Toyota",
			Model:        "Corolla",
			Year:         2020,
			Price:        90000 + i*2000,
		})
	}

	mc := NewMarketCache(data)
	median, _, cohort, ok := mc.Lookup("Toyota", "Corolla", 2020)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cohort != 15 {
		t.Errorf("expected cohort=15, got %d", cohort)
	}
	if median != 104000 {
		t.Errorf("expected median=104000, got %d", median)
	}
}

func TestMarketCache_LookupInsufficientData(t *testing.T) {
	data := []ListingData{
		{Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 100000},
		{Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 110000},
	}

	mc := NewMarketCache(data)
	_, _, _, ok := mc.Lookup("Toyota", "Corolla", 2020)
	if ok {
		t.Error("expected ok=false for insufficient data")
	}
}

func TestMarketCache_LookupYearBand(t *testing.T) {
	var data []ListingData
	for i := range 12 {
		data = append(data, ListingData{
			Manufacturer: "Honda",
			Model:        "Civic",
			Year:         2019 + (i % 3), // years 2019, 2020, 2021
			Price:        80000 + i*1000,
		})
	}

	mc := NewMarketCache(data)
	// Looking up year 2020 should include 2019, 2020, 2021
	_, _, cohort, ok := mc.Lookup("Honda", "Civic", 2020)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cohort != 12 {
		t.Errorf("expected cohort=12, got %d", cohort)
	}

	// Looking up year 2022 should only include 2021 (year+-1)
	_, _, _, ok = mc.Lookup("Honda", "Civic", 2022)
	if ok {
		t.Error("expected ok=false for year 2022 (only 4 listings in range)")
	}
}

func TestMarketCache_CaseInsensitive(t *testing.T) {
	var data []ListingData
	for i := range 10 {
		data = append(data, ListingData{
			Manufacturer: "TOYOTA",
			Model:        "COROLLA",
			Year:         2020,
			Price:        100000 + i*1000,
		})
	}

	mc := NewMarketCache(data)
	_, _, _, ok := mc.Lookup("toyota", "corolla", 2020)
	if !ok {
		t.Error("expected case-insensitive lookup to work")
	}
}

func TestMarketCache_Empty(t *testing.T) {
	mc := NewMarketCache(nil)
	_, _, _, ok := mc.Lookup("Toyota", "Corolla", 2020)
	if ok {
		t.Error("expected ok=false for empty cache")
	}
}

func TestMarketCache_MedianEven(t *testing.T) {
	var data []ListingData
	for i := range 10 {
		data = append(data, ListingData{
			Manufacturer: "Mazda",
			Model:        "3",
			Year:         2021,
			Price:        (i + 1) * 10000, // 10k, 20k, ..., 100k
		})
	}

	mc := NewMarketCache(data)
	median, _, _, ok := mc.Lookup("Mazda", "3", 2021)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Even count: average of 5th (50000) and 6th (60000) = 55000
	if median != 55000 {
		t.Errorf("expected median=55000, got %d", median)
	}
}

func TestCalibrationProfiles(t *testing.T) {
	stubCurrentYear(t, 2026)
	tests := []struct {
		name string
		p    FitnessParams
		min  float64
		max  float64
	}{
		{
			name: "candy car — 2018 67K 2nd hand fair price",
			p: FitnessParams{
				Price: 80000, Km: 67000, Hand: 2, Year: 2018, EngineVolume: 2000,
				PriceMax: 85000, EngineMinCC: 1800,
				MedianPrice: 77000, MedianKm: 110000,
			},
			min: 7.5, max: 8.5,
		},
		{
			name: "sweet spot — 2018 55K 2nd hand good price",
			p: FitnessParams{
				Price: 72000, Km: 55000, Hand: 2, Year: 2018, EngineVolume: 2000,
				PriceMax: 85000, EngineMinCC: 1800,
				MedianPrice: 77000, MedianKm: 110000,
			},
			min: 8.5, max: 10.0,
		},
		{
			name: "low-km gem — 2017 40K 1st hand",
			p: FitnessParams{
				Price: 78000, Km: 40000, Hand: 1, Year: 2017, EngineVolume: 2000,
				PriceMax: 85000, EngineMinCC: 1800,
				MedianPrice: 77000, MedianKm: 110000,
			},
			min: 9.0, max: 10.0,
		},
		{
			name: "average find — 2019 90K 2nd hand near cap",
			p: FitnessParams{
				Price: 82000, Km: 90000, Hand: 2, Year: 2019, EngineVolume: 2000,
				PriceMax: 85000, EngineMinCC: 1800,
				MedianPrice: 90000, MedianKm: 95000,
			},
			min: 5.0, max: 6.0,
		},
		{
			name: "overpriced mediocre — 2019 100K 3rd hand near cap",
			p: FitnessParams{
				Price: 84000, Km: 100000, Hand: 3, Year: 2019, EngineVolume: 2000,
				PriceMax: 85000, EngineMinCC: 1800,
				MedianPrice: 90000, MedianKm: 95000,
			},
			min: 3.0, max: 4.5,
		},
		{
			name: "high-km beater — 2017 160K 3rd hand cheap",
			p: FitnessParams{
				Price: 58000, Km: 160000, Hand: 3, Year: 2017, EngineVolume: 2000,
				PriceMax: 85000, EngineMinCC: 1800,
				MedianPrice: 65000, MedianKm: 130000,
			},
			min: 2.0, max: 3.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FitnessScore(tt.p)
			if got < tt.min || got > tt.max {
				t.Errorf("FitnessScore() = %.1f, want [%.1f, %.1f]", got, tt.min, tt.max)
			}
		})
	}
}

func TestFitnessScore_Monotonic(t *testing.T) {
	stubCurrentYear(t, 2026)
	base := FitnessParams{
		Price: 100000, Km: 80000, Hand: 2, Year: 2021, EngineVolume: 2000,
		PriceMax: 200000, EngineMinCC: 1500,
	}

	better := base
	better.Price = 80000
	better.Km = 40000
	better.Hand = 1
	better.Year = 2023

	baseScore := FitnessScore(base)
	betterScore := FitnessScore(better)

	if betterScore <= baseScore {
		t.Errorf("better listing (%.1f) should score higher than base (%.1f)", betterScore, baseScore)
	}
}

func TestFitnessScoreDetailed_MatchesTotal(t *testing.T) {
	stubCurrentYear(t, 2026)
	p := FitnessParams{
		Price: 100000, Km: 75000, Hand: 2, Year: 2021, EngineVolume: 2000,
		PriceMax: 200000, EngineMinCC: 1500,
	}

	simple := FitnessScore(p)
	detailed := FitnessScoreDetailed(p)

	if simple != detailed.Total {
		t.Errorf("FitnessScore()=%.1f != FitnessScoreDetailed().Total=%.1f", simple, detailed.Total)
	}
	if len(detailed.Dims) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(detailed.Dims))
	}
	for _, d := range detailed.Dims {
		if d.Score < 0 || d.Score > 1 {
			t.Errorf("dim %q score %.3f out of [0,1]", d.Name, d.Score)
		}
	}
}

func TestFitnessScoreDetailed_DimNames(t *testing.T) {
	stubCurrentYear(t, 2026)
	p := FitnessParams{
		Price: 80000, Km: 50000, Hand: 2, Year: 2020, EngineVolume: 2000,
		PriceMax: 150000, EngineMinCC: 1500,
	}
	result := FitnessScoreDetailed(p)
	want := map[string]bool{"condition": true, "value": true, "engine": true}
	for _, d := range result.Dims {
		if !want[d.Name] {
			t.Errorf("unexpected dimension name %q", d.Name)
		}
		delete(want, d.Name)
	}
	for name := range want {
		t.Errorf("missing dimension %q", name)
	}
}

func TestFitnessScore_EngineHardGate(t *testing.T) {
	stubCurrentYear(t, 2026)
	p := FitnessParams{
		Price: 80000, Km: 50000, Hand: 1, Year: 2022, EngineVolume: 1200,
		PriceMax: 200000, EngineMinCC: 1500,
	}
	got := FitnessScore(p)
	if got != 0 {
		t.Errorf("engine below minimum should gate total to 0, got %.1f", got)
	}
}

func TestFitnessScore_Range(t *testing.T) {
	stubCurrentYear(t, 2026)
	params := []FitnessParams{
		{Price: 1, Km: 999999, Hand: 10, Year: 2000, PriceMax: 1},
		{Price: 0, Km: 0, Hand: 0, Year: 2024, PriceMax: 0},
		{Price: 50000, Km: 50000, Hand: 2, Year: 2022, PriceMax: 200000},
		{Price: 100000, Km: 30000, Hand: 1, Year: 2023, PriceMax: 150000, MedianPrice: 120000, MedianKm: 60000},
	}
	for _, p := range params {
		s := FitnessScore(p)
		if s < 0 || s > 10 {
			t.Errorf("FitnessScore out of range [0,10]: %.1f for %+v", s, p)
		}
	}
}

func TestDefaultDimensions(t *testing.T) {
	dims := DefaultDimensions()
	if len(dims) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(dims))
	}
	want := []struct {
		name   string
		weight float64
	}{
		{"condition", condWeight},
		{"value", valWeight},
		{"engine", engineWeight},
	}
	for i := range want {
		if dims[i].Name != want[i].name {
			t.Errorf("dim %d name: got %q want %q", i, dims[i].Name, want[i].name)
		}
		if dims[i].Weight != want[i].weight {
			t.Errorf("dim %d weight: got %v want %v", i, dims[i].Weight, want[i].weight)
		}
	}
}

func TestComputeKmDelta(t *testing.T) {
	tests := []struct {
		name     string
		km       int
		carAge   int
		minDelta float64
		maxDelta float64
	}{
		{"zero km neutral", 0, 5, -0.01, 0.01},
		{"very low km bonus", 10000, 8, 3.0, 4.5},
		{"half expected bonus", 60000, 8, 1.0, 3.5},
		{"near expected dead zone", 120000, 8, -0.35, 0.0},
		{"above expected penalty", 160000, 8, -3.0, -0.3},
		{"extreme excess penalty", 300000, 8, -7.0, -2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, score01 := computeKmDelta(tt.km, tt.carAge)
			if delta < tt.minDelta || delta > tt.maxDelta {
				t.Errorf("computeKmDelta(%d, %d) delta=%.3f, want [%.1f, %.1f]", tt.km, tt.carAge, delta, tt.minDelta, tt.maxDelta)
			}
			if tt.km > 0 && (score01 < 0 || score01 > 1) {
				t.Errorf("score01=%.3f out of [0,1]", score01)
			}
		})
	}
}

func TestComputeHandDelta(t *testing.T) {
	tests := []struct {
		name     string
		hand     int
		carAge   int
		minDelta float64
		maxDelta float64
	}{
		{"unknown hand neutral", 0, 5, -0.01, 0.01},
		{"first hand new car", 1, 2, 0.6, 0.8},
		{"first hand old car", 1, 9, 1.2, 1.4},
		{"second hand expected", 2, 5, -0.1, 0.3},
		{"third hand excess", 3, 5, -1.5, -0.2},
		{"fourth hand excess", 4, 5, -3.0, -1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, _ := computeHandDelta(tt.hand, tt.carAge)
			if delta < tt.minDelta || delta > tt.maxDelta {
				t.Errorf("computeHandDelta(%d, %d) delta=%.3f, want [%.1f, %.1f]", tt.hand, tt.carAge, delta, tt.minDelta, tt.maxDelta)
			}
		})
	}
}

func TestComputeValueDelta_MarketMode(t *testing.T) {
	stubCurrentYear(t, 2026)

	t.Run("below market bonus", func(t *testing.T) {
		p := FitnessParams{
			Price: 65000, Year: 2019, Km: 40000, Hand: 1,
			MedianPrice: 90000, MedianKm: 95000, PriceMax: 150000,
		}
		delta, _ := computeValueDelta(p, 7, 0.8)
		if delta < 1.0 {
			t.Errorf("below-market price delta=%.3f, want >= 1.0", delta)
		}
	})

	t.Run("at market neutral", func(t *testing.T) {
		p := FitnessParams{
			Price: 90000, Year: 2019, Km: 95000, Hand: 2,
			MedianPrice: 90000, MedianKm: 95000, PriceMax: 150000,
		}
		delta, _ := computeValueDelta(p, 7, 0.5)
		if delta < -0.5 || delta > 0.5 {
			t.Errorf("at-market price delta=%.3f, want near zero", delta)
		}
	})

	t.Run("above market penalty", func(t *testing.T) {
		p := FitnessParams{
			Price: 110000, Year: 2019, Km: 95000, Hand: 2,
			MedianPrice: 90000, MedianKm: 95000, PriceMax: 150000,
		}
		delta, _ := computeValueDelta(p, 7, 0.5)
		if delta > -0.3 {
			t.Errorf("above-market price delta=%.3f, want <= -0.3", delta)
		}
	})

	t.Run("condition gate scales down bonus", func(t *testing.T) {
		p := FitnessParams{
			Price: 60000, Year: 2017, Km: 160000, Hand: 3,
			MedianPrice: 80000, MedianKm: 100000, PriceMax: 150000,
		}
		ungated, _ := computeValueDelta(p, 9, 0.80)
		gated, _ := computeValueDelta(p, 9, 0.30)
		if gated >= ungated {
			t.Errorf("condition gate should reduce bonus: gated=%.3f ungated=%.3f", gated, ungated)
		}
	})
}

func TestComputeValueDelta_BudgetFallback(t *testing.T) {
	t.Run("lots of headroom", func(t *testing.T) {
		p := FitnessParams{Price: 50000, PriceMax: 100000}
		delta, _ := computeValueDelta(p, 5, 0.5)
		if delta < 1.5 {
			t.Errorf("50%% headroom delta=%.3f, want >= 1.5", delta)
		}
	})

	t.Run("at budget cap", func(t *testing.T) {
		p := FitnessParams{Price: 100000, PriceMax: 100000}
		delta, _ := computeValueDelta(p, 5, 0.5)
		if delta > 0 {
			t.Errorf("at-cap delta=%.3f, want <= 0", delta)
		}
	})

	t.Run("no price and no budget", func(t *testing.T) {
		p := FitnessParams{Price: 0, PriceMax: 0}
		delta, _ := computeValueDelta(p, 5, 0.5)
		if delta != 0 {
			t.Errorf("no price/budget delta=%.3f, want 0", delta)
		}
	})
}

func TestComputeEngineDelta(t *testing.T) {
	tests := []struct {
		name    string
		volume  float64
		minCC   int
		wantMin float64
		wantMax float64
		score01 float64
	}{
		{"no minimum", 1600, 0, 0, 0, 1.0},
		{"unknown volume", 0, 1600, 0, 0, 0.5},
		{"below minimum", 1200, 1600, 0, 0, 0},
		{"at minimum", 1600, 1600, 0, 0.01, 0.7},
		{"above minimum", 2000, 1600, 0.01, 0.1, 0.85},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, score01 := computeEngineDelta(tt.volume, tt.minCC)
			if delta < tt.wantMin-0.01 || delta > tt.wantMax+0.01 {
				t.Errorf("delta=%.4f, want [%.2f, %.2f]", delta, tt.wantMin, tt.wantMax)
			}
			if math.Abs(score01-tt.score01) > 0.1 {
				t.Errorf("score01=%.3f, want ~%.1f", score01, tt.score01)
			}
		})
	}
}

func TestScoreWithKm(t *testing.T) {
	priceOnly := Score(80000, 100000) // 20% below median -> score 20
	lowKm := ScoreWithKm(80000, 30000, 100000, 80000)
	highKm := ScoreWithKm(80000, 150000, 100000, 80000)

	if lowKm <= priceOnly {
		t.Errorf("low-km deal (%d) should score higher than price-only (%d)", lowKm, priceOnly)
	}
	if highKm >= priceOnly {
		t.Errorf("high-km deal (%d) should score lower than price-only (%d)", highKm, priceOnly)
	}
}

func TestScoreWithKm_UnknownKmPenalty(t *testing.T) {
	priceOnly := Score(80000, 100000)
	noKm := ScoreWithKm(80000, 0, 100000, 80000)
	wantPenalized := priceOnly - 5
	if noKm != wantPenalized {
		t.Errorf("ScoreWithKm with listingKm=0 and cohort km: want %d, got %d", wantPenalized, noKm)
	}
}

func TestScoreWithKm_FallbackWhenNoCohortKm(t *testing.T) {
	priceOnly := Score(80000, 100000)
	noMedianKm := ScoreWithKm(80000, 50000, 100000, 0)
	if noMedianKm != priceOnly {
		t.Errorf("ScoreWithKm with medianKm=0 should equal Score: %d != %d", noMedianKm, priceOnly)
	}
}

func TestScoreWithKm_UnknownKmPenaltyFloorsAtZero(t *testing.T) {
	got := ScoreWithKm(96000, 0, 100000, 50000)
	if got != 0 {
		t.Errorf("penalized score should floor at 0, got %d", got)
	}
}

func TestMarketCache_MedianKmRequiresMinSamples(t *testing.T) {
	// 10 valid prices but only 2 non-zero km -> medianKm must be 0.
	var data []ListingData
	for i := range 8 {
		data = append(data, ListingData{
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020,
			Price: 90000 + i*2000, Km: 0,
		})
	}
	data = append(data,
		ListingData{Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 106000, Km: 40000},
		ListingData{Manufacturer: "Toyota", Model: "Corolla", Year: 2020, Price: 108000, Km: 60000},
	)

	mc := NewMarketCache(data)
	_, medianKm, cohort, ok := mc.Lookup("Toyota", "Corolla", 2020)
	if !ok || cohort != 10 {
		t.Fatalf("lookup: ok=%v cohort=%d", ok, cohort)
	}
	if medianKm != 0 {
		t.Errorf("medianKm should be 0 with <3 non-zero km samples, got %d", medianKm)
	}
}

func TestMarketCache_MedianKmWithThreeSamples(t *testing.T) {
	var data []ListingData
	for i := range 7 {
		data = append(data, ListingData{
			Manufacturer: "Toyota", Model: "Camry", Year: 2020,
			Price: 90000 + i*2000, Km: 0,
		})
	}
	data = append(data,
		ListingData{Manufacturer: "Toyota", Model: "Camry", Year: 2020, Price: 104000, Km: 30000},
		ListingData{Manufacturer: "Toyota", Model: "Camry", Year: 2020, Price: 106000, Km: 50000},
		ListingData{Manufacturer: "Toyota", Model: "Camry", Year: 2020, Price: 108000, Km: 70000},
	)

	mc := NewMarketCache(data)
	_, medianKm, cohort, ok := mc.Lookup("Toyota", "Camry", 2020)
	if !ok || cohort != 10 {
		t.Fatalf("lookup: ok=%v cohort=%d", ok, cohort)
	}
	if medianKm != 50000 {
		t.Errorf("medianKm want 50000, got %d", medianKm)
	}
}

func TestJunkPriceFiltering(t *testing.T) {
	var data []ListingData
	// 10 legitimate prices
	for i := range 10 {
		data = append(data, ListingData{
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020,
			Price: 90000 + i*2000,
		})
	}
	// 3 junk prices that should be filtered
	for range 3 {
		data = append(data, ListingData{
			Manufacturer: "Toyota", Model: "Corolla", Year: 2020,
			Price: 1, // placeholder
		})
	}

	mc := NewMarketCache(data)
	median, _, cohort, ok := mc.Lookup("Toyota", "Corolla", 2020)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cohort != 10 {
		t.Errorf("junk prices should be excluded: cohort=%d, want 10", cohort)
	}
	if median < 90000 || median > 108000 {
		t.Errorf("median should be in legitimate range, got %d", median)
	}
}

func TestMarketCache_LookupConcurrent(t *testing.T) {
	var data []ListingData
	for i := range 15 {
		data = append(data, ListingData{
			Manufacturer: "Toyota",
			Model:        "Corolla",
			Year:         2020,
			Price:        90000 + i*2000,
		})
	}
	mc := NewMarketCache(data)
	var wg sync.WaitGroup
	var bad atomic.Int32
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			median, _, cohort, ok := mc.Lookup("Toyota", "Corolla", 2020)
			if !ok || cohort != 15 || median != 104000 {
				bad.Add(1)
			}
		}()
	}
	wg.Wait()
	if bad.Load() != 0 {
		t.Fatalf("concurrent lookups: %d mismatches", bad.Load())
	}
}

func TestNewMarketCacheFromMedians(t *testing.T) {
	entries := []MedianEntry{
		{Manufacturer: "Toyota", Model: "Corolla", Year: 2020, MedianPrice: 105000, MedianKm: 55000, CohortSize: 25},
		{Manufacturer: "Honda", Model: "Civic", Year: 2021, MedianPrice: 98000, MedianKm: 40000, CohortSize: 18},
	}
	mc := NewMarketCacheFromMedians(entries)

	t.Run("exact match", func(t *testing.T) {
		median, medianKm, cohort, ok := mc.Lookup("Toyota", "Corolla", 2020)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if median != 105000 {
			t.Errorf("median=%d, want 105000", median)
		}
		if medianKm != 55000 {
			t.Errorf("medianKm=%d, want 55000", medianKm)
		}
		if cohort != 25 {
			t.Errorf("cohort=%d, want 25", cohort)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		_, _, _, ok := mc.Lookup("toyota", "corolla", 2020)
		if !ok {
			t.Error("expected case-insensitive lookup to work")
		}
	})

	t.Run("year band matching", func(t *testing.T) {
		// Precomputed path uses ±1 year band, same as raw path.
		median, _, cohort, ok := mc.Lookup("Toyota", "Corolla", 2021)
		if !ok {
			t.Fatal("expected ok=true for adjacent year (2021 within ±1 of 2020)")
		}
		if median != 105000 {
			t.Errorf("median=%d, want 105000", median)
		}
		if cohort != 25 {
			t.Errorf("cohort=%d, want 25", cohort)
		}
	})

	t.Run("year too far", func(t *testing.T) {
		_, _, _, ok := mc.Lookup("Toyota", "Corolla", 2023)
		if ok {
			t.Error("expected ok=false for year 2023 (>1 from 2020)")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, _, _, ok := mc.Lookup("Mazda", "3", 2020)
		if ok {
			t.Error("expected ok=false for missing manufacturer/model")
		}
	})

	t.Run("below min cohort", func(t *testing.T) {
		small := []MedianEntry{
			{Manufacturer: "Kia", Model: "Rio", Year: 2020, MedianPrice: 60000, MedianKm: 30000, CohortSize: 5},
		}
		mc2 := NewMarketCacheFromMedians(small)
		_, _, _, ok := mc2.Lookup("Kia", "Rio", 2020)
		if ok {
			t.Error("expected ok=false when cohort < MinCohortSize")
		}
	})

	t.Run("empty entries", func(t *testing.T) {
		mc3 := NewMarketCacheFromMedians(nil)
		_, _, _, ok := mc3.Lookup("Toyota", "Corolla", 2020)
		if ok {
			t.Error("expected ok=false for empty cache")
		}
	})

	t.Run("concurrent reads", func(t *testing.T) {
		var wg sync.WaitGroup
		var bad atomic.Int32
		for range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				median, _, cohort, ok := mc.Lookup("Honda", "Civic", 2021)
				if !ok || cohort != 18 || median != 98000 {
					bad.Add(1)
				}
			}()
		}
		wg.Wait()
		if bad.Load() != 0 {
			t.Fatalf("concurrent lookups: %d mismatches", bad.Load())
		}
	})
}

// stubCurrentYear overrides the currentYear func for deterministic scoring tests.
func stubCurrentYear(t *testing.T, year int) {
	t.Helper()
	orig := currentYear
	currentYear = func() int { return year }
	t.Cleanup(func() { currentYear = orig })
}

func TestHondaAccordBES(t *testing.T) {
	stubCurrentYear(t, 2026)

	p := FitnessParams{
		Price: 39999, Km: 82000, Hand: 2, Year: 2014, EngineVolume: 2000,
		PriceMax: 50000, EngineMinCC: 0,
	}
	result := FitnessScoreDetailed(p)

	if result.Total < 7.0 || result.Total > 10.0 {
		t.Errorf("Honda Accord total = %.1f, want [7.0, 10.0]", result.Total)
	}
}

func TestFitnessScore_MarketVsBudget(t *testing.T) {
	stubCurrentYear(t, 2026)

	base := FitnessParams{
		Price: 100000, Km: 50000, Hand: 2, Year: 2022, EngineVolume: 2000,
		PriceMax: 200000, EngineMinCC: 1500,
	}

	withMarket := base
	withMarket.MedianPrice = 130000
	withMarket.MedianKm = 70000

	noMarket := base

	marketScore := FitnessScore(withMarket)
	budgetScore := FitnessScore(noMarket)

	if marketScore < budgetScore-1.0 {
		t.Errorf("below-market (%.1f) should not be much worse than budget-only (%.1f)", marketScore, budgetScore)
	}
}

func TestFitnessScore_BudgetFallback(t *testing.T) {
	stubCurrentYear(t, 2026)

	p := FitnessParams{
		Price: 100000, Km: 50000, Hand: 2, Year: 2022, EngineVolume: 2000,
		PriceMax: 200000, EngineMinCC: 1500, MedianPrice: 0,
	}
	result := FitnessScoreDetailed(p)

	dims := make(map[string]float64)
	for _, d := range result.Dims {
		dims[d.Name] = d.Score
	}
	if dims["value"] < 0.4 || dims["value"] > 1.0 {
		t.Errorf("budget fallback value dim = %.3f, want [0.4, 1.0]", dims["value"])
	}
}

func TestKmDelta_AgeAdjusted(t *testing.T) {
	// Same absolute km but different car ages should produce very different deltas.
	oldDelta, _ := computeKmDelta(80000, 12) // ~6.7k/yr
	newDelta, _ := computeKmDelta(80000, 2)  // ~40k/yr

	if oldDelta <= newDelta {
		t.Errorf("80k km: old car delta %.3f should exceed new car delta %.3f", oldDelta, newDelta)
	}
	if oldDelta < 0 {
		t.Errorf("80k on 12yr car should be positive delta, got %.3f", oldDelta)
	}
	if newDelta > 0 {
		t.Errorf("80k on 2yr car should be negative delta, got %.3f", newDelta)
	}
}

func TestHandDelta_AgeScaling(t *testing.T) {
	// First hand on old car should get bigger bonus than on new car.
	oldDelta, _ := computeHandDelta(1, 12)
	newDelta, _ := computeHandDelta(1, 2)

	if oldDelta <= newDelta {
		t.Errorf("first hand: old car delta %.3f should exceed new car delta %.3f", oldDelta, newDelta)
	}
}

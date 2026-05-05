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

func TestFitnessScore(t *testing.T) {
	tests := []struct {
		name string
		p    FitnessParams
		min  float64
		max  float64
	}{
		{
			name: "perfect listing",
			p: FitnessParams{
				Price: 50000, Km: 1000, Hand: 1, Year: 2024, EngineVolume: 3000,
				PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 9.0, max: 10.0,
		},
		{
			name: "worst listing within filters",
			p: FitnessParams{
				Price: 200000, Km: 150000, Hand: 4, Year: 2018, EngineVolume: 1500,
				PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 0.0, max: 1.5,
		},
		{
			name: "middle of the road",
			p: FitnessParams{
				Price: 100000, Km: 75000, Hand: 2, Year: 2021, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 4.5, max: 6.5,
		},
		{
			name: "PriceMax=0 excludes price dimension",
			p: FitnessParams{
				Price: 999999, Km: 10000, Hand: 1, Year: 2024, EngineVolume: 2000,
				PriceMax: 0, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 8.0, max: 10.0,
		},
		{
			name: "Price=0 excludes price dimension",
			p: FitnessParams{
				Price: 0, Km: 10000, Hand: 1, Year: 2024, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 8.0, max: 10.0,
		},
		{
			name: "MaxKm=0 uses absolute 200k scale",
			p: FitnessParams{
				Price: 100000, Km: 100000, Hand: 2, Year: 2021, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 0, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 4.0, max: 6.5,
		},
		{
			name: "MaxHand=0 uses absolute scale",
			p: FitnessParams{
				Price: 100000, Km: 50000, Hand: 1, Year: 2022, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 150000, MaxHand: 0, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 6.0, max: 8.5,
		},
		{
			name: "single year range gives full year score",
			p: FitnessParams{
				Price: 100000, Km: 50000, Hand: 2, Year: 2022, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2022, YearMax: 2022, EngineMinCC: 1500,
			},
			min: 5.5, max: 7.5,
		},
		{
			name: "EngineMinCC=0 gives full engine score",
			p: FitnessParams{
				Price: 100000, Km: 50000, Hand: 2, Year: 2022, EngineVolume: 1200,
				PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 0,
			},
			min: 5.0, max: 7.0,
		},
		{
			name: "all criteria are any",
			p: FitnessParams{
				Price: 0, Km: 50000, Hand: 2, Year: 2022, EngineVolume: 2000,
				PriceMax: 0, MaxKm: 0, MaxHand: 0, YearMin: 0, YearMax: 0, EngineMinCC: 0,
			},
			min: 6.0, max: 9.0,
		},
		{
			name: "unknown km omits km dimension",
			p: FitnessParams{
				Price: 150000, Km: 0, Hand: 1, Year: 2024, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 100000, MaxHand: 3, YearMin: 2020, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 6.5, max: 7.5,
		},
		{
			name: "price exactly at max",
			p: FitnessParams{
				Price: 200000, Km: 0, Hand: 1, Year: 2024, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 100000, MaxHand: 3, YearMin: 2020, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 4.0, max: 6.5,
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
	base := FitnessParams{
		Price: 100000, Km: 80000, Hand: 2, Year: 2021, EngineVolume: 2000,
		PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
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
	p := FitnessParams{
		Price: 100000, Km: 75000, Hand: 2, Year: 2021, EngineVolume: 2000,
		PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
	}

	simple := FitnessScore(p)
	detailed := FitnessScoreDetailed(p)

	if simple != detailed.Total {
		t.Errorf("FitnessScore()=%.1f != FitnessScoreDetailed().Total=%.1f", simple, detailed.Total)
	}
	if len(detailed.Dims) != 5 {
		t.Errorf("expected 5 dimensions, got %d", len(detailed.Dims))
	}
	for _, d := range detailed.Dims {
		if d.Score < 0 || d.Score > 1 {
			t.Errorf("dim %q score %.3f out of [0,1]", d.Name, d.Score)
		}
	}
}

func TestFitnessScoreDetailed_NoPriceDim(t *testing.T) {
	p := FitnessParams{
		Price: 0, Km: 50000, Hand: 1, Year: 2024, EngineVolume: 2000,
		PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
	}
	result := FitnessScoreDetailed(p)
	for _, d := range result.Dims {
		if d.Name == "price" {
			t.Error("price dimension should be excluded when Price=0")
		}
	}
	if len(result.Dims) != 4 {
		t.Errorf("expected 4 dimensions without price, got %d", len(result.Dims))
	}
}

func TestKmScore(t *testing.T) {
	tests := []struct {
		name    string
		km      int
		maxKm   int
		wantNaN bool
		want    float64
	}{
		{"zero km omits dimension", 0, 150000, true, 0},
		{"negative km omits dimension", -1, 150000, true, 0},
		{"at max km scores zero", 150000, 150000, false, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kmScore(tt.km, tt.maxKm)
			if tt.wantNaN {
				if !math.IsNaN(got) {
					t.Errorf("kmScore(%d, %d) = %v, want NaN", tt.km, tt.maxKm, got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("kmScore(%d, %d) = %.2f, want %.2f", tt.km, tt.maxKm, got, tt.want)
			}
		})
	}

	t.Run("known low vs unknown", func(t *testing.T) {
		low := kmScore(10000, 150000)
		unknown := kmScore(0, 150000)
		if !math.IsNaN(unknown) {
			t.Fatalf("unknown km should be NaN, got %v", unknown)
		}
		if low <= 0 {
			t.Errorf("low-km should score positive, got %.3f", low)
		}
	})
}

func TestFitnessScore_NonLinearKm(t *testing.T) {
	base := FitnessParams{
		Price: 100000, Hand: 2, Year: 2021, EngineVolume: 2000,
		PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
	}

	low := base
	low.Km = 20000
	mid := base
	mid.Km = 60000

	lowScore := FitnessScoreDetailed(low)
	midScore := FitnessScoreDetailed(mid)

	var lowKm, midKm float64
	for _, d := range lowScore.Dims {
		if d.Name == "km" {
			lowKm = d.Score
		}
	}
	for _, d := range midScore.Dims {
		if d.Name == "km" {
			midKm = d.Score
		}
	}

	gap := lowKm - midKm
	if gap < 0.20 {
		t.Errorf("non-linear km: 20k vs 60k gap=%.3f, want >= 0.20 (low km should be strongly rewarded)", gap)
	}
}

func TestFitnessScore_Range(t *testing.T) {
	params := []FitnessParams{
		{Price: 1, Km: 999999, Hand: 10, Year: 2000, PriceMax: 1, MaxKm: 1, MaxHand: 1, YearMin: 2020, YearMax: 2024},
		{Price: 0, Km: 0, Hand: 0, Year: 2024, PriceMax: 0, MaxKm: 0, MaxHand: 0, YearMin: 0, YearMax: 0},
		{Price: 50000, Km: 50000, Hand: 2, Year: 2022, PriceMax: 200000, MaxKm: 150000, MaxHand: 4, YearMin: 2018, YearMax: 2024},
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
	if len(dims) != 5 {
		t.Fatalf("expected 5 dimensions, got %d", len(dims))
	}
	want := []struct {
		name   string
		weight float64
	}{
		{"price", weightPrice},
		{"km", weightKm},
		{"hand", weightHand},
		{"year", weightYear},
		{"engine", weightEngine},
	}
	for i := range want {
		if dims[i].Name != want[i].name {
			t.Errorf("dim %d name: got %q want %q", i, dims[i].Name, want[i].name)
		}
		if dims[i].Weight != want[i].weight {
			t.Errorf("dim %d weight: got %v want %v", i, dims[i].Weight, want[i].weight)
		}
		if dims[i].Score == nil {
			t.Errorf("dim %d Score is nil", i)
		}
	}
}

func TestEngineScore(t *testing.T) {
	tests := []struct {
		name   string
		volume float64
		minCC  int
		min    float64
		max    float64
	}{
		{"no minimum set", 1600, 0, 1.0, 1.0},
		{"unknown volume", 0, 1600, 0.5, 0.5},
		{"below minimum", 1200, 1600, 0.0, 0.0},
		{"exactly at minimum", 1600, 1600, 0.7, 0.7},
		{"2.0L with 1.6L minimum", 2000, 1600, 0.85, 1.0},
		{"2.4L with 1.6L minimum", 2400, 1600, 1.0, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engineScore(tt.volume, tt.minCC)
			if got < tt.min-0.01 || got > tt.max+0.01 {
				t.Errorf("engineScore(%.0f, %d) = %.3f, want [%.2f, %.2f]", tt.volume, tt.minCC, got, tt.min, tt.max)
			}
		})
	}
}

func TestYearScore_NonLinear(t *testing.T) {
	// Newer years should cluster near the top (concave curve).
	// 2023 vs 2024 gap should be smaller than 2018 vs 2019 gap.
	scoreNewer := yearScore(2023, 2018, 2024) // near top
	scoreOlder := yearScore(2019, 2018, 2024) // near bottom

	// With pow(0.7), the spread between adjacent years at the top is smaller.
	topGap := 1.0 - scoreNewer
	bottomGap := scoreOlder - 0.0

	if topGap >= bottomGap {
		t.Errorf("concave curve should make top gap (%.3f) smaller than bottom gap (%.3f)", topGap, bottomGap)
	}
}

func TestYearScore_BelowMin(t *testing.T) {
	score := yearScore(2015, 2018, 2024)
	if score != 0.0 {
		t.Errorf("year below min should score 0, got %.3f", score)
	}
	if math.IsNaN(score) {
		t.Fatal("yearScore returned NaN for year below min")
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

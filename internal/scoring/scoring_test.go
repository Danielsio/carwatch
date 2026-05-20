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
	stubCurrentYear(t, 2026)
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
			min: 6.5, max: 8.5,
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
	stubCurrentYear(t, 2026)
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
	stubCurrentYear(t, 2026)
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
	stubCurrentYear(t, 2026)
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
	stubCurrentYear(t, 2026)
	tests := []struct {
		name    string
		km      int
		maxKm   int
		year    int
		wantNaN bool
		wantMin float64
		wantMax float64
	}{
		{"zero km omits dimension", 0, 150000, 2020, true, 0, 0},
		{"negative km omits dimension", -1, 150000, 2020, true, 0, 0},
		{"at max km scores near zero", 150000, 150000, 2020, false, 0.0, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kmScore(tt.km, tt.maxKm, tt.year)
			if tt.wantNaN {
				if !math.IsNaN(got) {
					t.Errorf("kmScore(%d, %d, %d) = %v, want NaN", tt.km, tt.maxKm, tt.year, got)
				}
				return
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("kmScore(%d, %d, %d) = %.4f, want [%.2f, %.2f]", tt.km, tt.maxKm, tt.year, got, tt.wantMin, tt.wantMax)
			}
		})
	}

	t.Run("known low vs unknown", func(t *testing.T) {
		low := kmScore(10000, 150000, 2022)
		unknown := kmScore(0, 150000, 2022)
		if !math.IsNaN(unknown) {
			t.Fatalf("unknown km should be NaN, got %v", unknown)
		}
		if low <= 0 {
			t.Errorf("low-km should score positive, got %.3f", low)
		}
	})

	t.Run("age-adjusted: low km on old car scores high", func(t *testing.T) {
		score := kmScore(82000, 200000, 2014)
		if score < 0.45 {
			t.Errorf("82k km on 2014 car should score well, got %.3f", score)
		}
	})

	t.Run("age-adjusted: high km on new car scores low", func(t *testing.T) {
		score := kmScore(82000, 200000, 2024)
		if score > 0.35 {
			t.Errorf("82k km on 2024 car should score poorly, got %.3f", score)
		}
	})
}

func TestFitnessScore_NonLinearKm(t *testing.T) {
	stubCurrentYear(t, 2026)
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
	stubCurrentYear(t, 2026)
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
	// sqrt curve: newer years still cluster near the top, older years spread apart.
	scoreNewer := yearScore(2023, 2018, 2024)
	scoreOlder := yearScore(2019, 2018, 2024)

	topGap := 1.0 - scoreNewer
	bottomGap := scoreOlder - yearScoreFloor

	if topGap >= bottomGap {
		t.Errorf("sqrt curve should make top gap (%.3f) smaller than bottom gap (%.3f)", topGap, bottomGap)
	}

	// Floor: even the lowest in-range year gets a meaningful score.
	minScore := yearScore(2018, 2018, 2024)
	if minScore < yearScoreFloor {
		t.Errorf("year at min should score at least floor (%.1f), got %.3f", yearScoreFloor, minScore)
	}
}

func TestYearScore_BelowMin(t *testing.T) {
	score := yearScore(2015, 2018, 2024)
	if score != yearScoreFloor {
		t.Errorf("year below min should score floor (%.1f), got %.3f", yearScoreFloor, score)
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

func TestHondaAccordScoring(t *testing.T) {
	stubCurrentYear(t, 2026)

	p := FitnessParams{
		Price: 39999, Km: 82000, Hand: 2, Year: 2014, EngineVolume: 2000,
		PriceMax: 50000, MaxKm: 0, MaxHand: 0, YearMin: 2012, YearMax: 2020, EngineMinCC: 0,
	}
	result := FitnessScoreDetailed(p)

	// With the value-oriented algorithm this should score ~6-7 ("good" range).
	if result.Total < 5.5 || result.Total > 7.5 {
		t.Errorf("Honda Accord total = %.1f, want [5.5, 7.5]", result.Total)
	}

	dims := make(map[string]float64)
	for _, d := range result.Dims {
		dims[d.Name] = d.Score
	}

	// Price (budget fallback, no MedianPrice): 39999/50000 = 0.80 ratio, sqrt(0.20) ≈ 0.45.
	if dims["price"] < 0.40 || dims["price"] > 0.55 {
		t.Errorf("price dim = %.3f, want [0.40, 0.55]", dims["price"])
	}

	// Km: 82k on 12-year car is ~7k/yr (well below 15k avg) — should score decently.
	if dims["km"] < 0.35 {
		t.Errorf("km dim = %.3f, want >= 0.35 (low wear for age)", dims["km"])
	}

	// Hand: 2 on a 12-year-old car with age bonus should be decent.
	if dims["hand"] < 0.75 {
		t.Errorf("hand dim = %.3f, want >= 0.75 (hand 2 on old car)", dims["hand"])
	}

	// Year: 2014 in 2012-2020 (pos=0.25) with floor at 0.3 and sqrt.
	if dims["year"] < 0.55 {
		t.Errorf("year dim = %.3f, want >= 0.55", dims["year"])
	}
}

func TestKmScore_AgeAdjusted(t *testing.T) {
	stubCurrentYear(t, 2026)

	// Same absolute km but different car ages should score very differently.
	oldCar := kmScore(80000, 150000, 2014) // 12yr, ~6.7k/yr
	newCar := kmScore(80000, 150000, 2024) // 2yr, ~40k/yr

	if oldCar <= newCar {
		t.Errorf("80k km: old car (2014) score %.3f should beat new car (2024) score %.3f", oldCar, newCar)
	}

	gap := oldCar - newCar
	if gap < 0.15 {
		t.Errorf("age adjustment gap = %.3f, want >= 0.15 (meaningful difference)", gap)
	}
}

func TestHandScore_AgeBonus(t *testing.T) {
	stubCurrentYear(t, 2026)

	// Hand 2 on old car should get a bonus vs hand 2 on new car.
	oldCar := handScore(2, 0, 2014) // 12yr old
	newCar := handScore(2, 0, 2024) // 2yr old

	if oldCar <= newCar {
		t.Errorf("hand 2: old car score %.3f should beat new car score %.3f", oldCar, newCar)
	}

	// Hand 1 should never get a bonus regardless of age.
	hand1old := handScore(1, 0, 2014)
	hand1new := handScore(1, 0, 2024)
	if hand1old != hand1new {
		t.Errorf("hand 1 should have no age bonus: old=%.3f new=%.3f", hand1old, hand1new)
	}
}

func TestKmScore_ZeroCarYear(t *testing.T) {
	stubCurrentYear(t, 2026)

	// carYear=0 should not inflate score — fall back to cap-only.
	withCap := kmScore(80000, 150000, 0)
	if math.IsNaN(withCap) {
		t.Fatal("kmScore with maxKm>0 and carYear=0 should not be NaN")
	}
	// Without age adjustment, score is purely cap-based.
	expectedCap := clamp01(1.0 - math.Pow(clamp01(80000.0/150000.0), kmCapExponent))
	if math.Abs(withCap-expectedCap) > 0.01 {
		t.Errorf("kmScore(80k, 150k, year=0) = %.3f, want cap-only %.3f", withCap, expectedCap)
	}

	// No maxKm and no year → NaN (no signal at all).
	noSignal := kmScore(80000, 0, 0)
	if !math.IsNaN(noSignal) {
		t.Errorf("kmScore(80k, maxKm=0, year=0) should be NaN, got %.3f", noSignal)
	}
}

func TestBudgetPriceScore(t *testing.T) {
	tests := []struct {
		name     string
		price    int
		priceMax int
		want     float64
		tol      float64
	}{
		{"priceMax=0 returns 0.5", 100000, 0, 0.5, 0},
		{"price at max returns 0", 200000, 200000, 0.0, 0},
		{"price over max returns 0", 250000, 200000, 0.0, 0},
		{"price at 50% of max", 100000, 200000, math.Sqrt(0.5), 0.01},
		{"price at 80% of max", 160000, 200000, math.Sqrt(0.2), 0.01},
		{"price at 0", 0, 200000, 1.0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := budgetPriceScore(tt.price, tt.priceMax)
			if math.Abs(got-tt.want) > tt.tol {
				t.Errorf("budgetPriceScore(%d, %d) = %.4f, want %.4f (±%.2f)", tt.price, tt.priceMax, got, tt.want, tt.tol)
			}
		})
	}
}

func TestMarketPriceScore(t *testing.T) {
	tests := []struct {
		name   string
		price  int
		median int
		want   float64
		tol    float64
	}{
		{"70% of median → 1.0 (exceptional deal)", 70000, 100000, 1.0, 0.01},
		{"85% of median → ~0.87 (good deal)", 85000, 100000, 0.87, 0.02},
		{"at median → ~0.71 (fair price)", 100000, 100000, math.Sqrt(0.5), 0.01},
		{"115% of median → 0.50 (above market)", 115000, 100000, 0.50, 0.02},
		{"130% of median → 0 (overpriced)", 130000, 100000, 0.0, 0.01},
		{"150% of median → 0 (clamped)", 150000, 100000, 0.0, 0.01},
		{"50% of median → 1.0 (clamped at floor)", 50000, 100000, 1.0, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marketPriceScore(tt.price, tt.median)
			if math.Abs(got-tt.want) > tt.tol {
				t.Errorf("marketPriceScore(%d, %d) = %.4f, want %.4f (±%.2f)", tt.price, tt.median, got, tt.want, tt.tol)
			}
		})
	}
}

func TestFitnessScore_MarketPricePreferred(t *testing.T) {
	stubCurrentYear(t, 2026)

	base := FitnessParams{
		Km: 50000, Hand: 2, Year: 2022, EngineVolume: 2000,
		PriceMax: 200000, MaxKm: 150000, MaxHand: 4,
		YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
	}

	// Car priced at 100k with median 130k → below market, good value.
	belowMarket := base
	belowMarket.Price = 100000
	belowMarket.MedianPrice = 130000

	// Same car priced at 100k with no market data → falls back to budget.
	noBudget := base
	noBudget.Price = 100000

	belowResult := FitnessScoreDetailed(belowMarket)
	budgetResult := FitnessScoreDetailed(noBudget)

	var belowPriceDim, budgetPriceDim float64
	for _, d := range belowResult.Dims {
		if d.Name == "price" {
			belowPriceDim = d.Score
		}
	}
	for _, d := range budgetResult.Dims {
		if d.Name == "price" {
			budgetPriceDim = d.Score
		}
	}

	// 100k/130k = 0.77 → market score ~0.88; 100k/200k = 0.5 → budget score ~0.71
	if belowPriceDim <= budgetPriceDim {
		t.Errorf("below-market car price dim (%.3f) should score higher than budget-only (%.3f)",
			belowPriceDim, budgetPriceDim)
	}
}

func TestFitnessScore_MarketPriceFallback(t *testing.T) {
	stubCurrentYear(t, 2026)

	// When MedianPrice=0, should fall back to budget-based scoring.
	p := FitnessParams{
		Price: 100000, Km: 50000, Hand: 2, Year: 2022, EngineVolume: 2000,
		PriceMax: 200000, MaxKm: 150000, MaxHand: 4,
		YearMin: 2018, YearMax: 2024, EngineMinCC: 1500,
		MedianPrice: 0,
	}
	result := FitnessScoreDetailed(p)
	var priceDim float64
	for _, d := range result.Dims {
		if d.Name == "price" {
			priceDim = d.Score
		}
	}
	// 100k/200k = 0.5 → sqrt(0.5) ≈ 0.707
	want := math.Sqrt(0.5)
	if math.Abs(priceDim-want) > 0.01 {
		t.Errorf("fallback price dim = %.4f, want %.4f (budget-based)", priceDim, want)
	}
}

func TestKmScore_AgeOnlyPath(t *testing.T) {
	stubCurrentYear(t, 2026)

	// km>0, maxKm=0, carYear>0 → pure ageScore (no blending).
	got := kmScore(60000, 0, 2020)
	if math.IsNaN(got) {
		t.Fatal("kmScore with km>0 and carYear>0 should not be NaN even without maxKm")
	}

	// Verify it matches the age-adjusted formula directly.
	carAge := 2026 - 2020
	expectedKm := float64(carAge * avgKmPerYear)
	kmRatio := 60000.0 / expectedKm
	wantAge := clamp01(1.0 - math.Pow(clamp01(kmRatio), kmAgeExponent))
	if math.Abs(got-wantAge) > 0.001 {
		t.Errorf("kmScore(60k, maxKm=0, 2020) = %.4f, want pure ageScore %.4f", got, wantAge)
	}
}

func TestHandScore_WithMaxHandAndAgeBonus(t *testing.T) {
	stubCurrentYear(t, 2026)

	// Hand 3, maxHand=4, carYear=2014 (age 12) → ratio-based + age bonus.
	got := handScore(3, 4, 2014)

	// Base: ratio = (3-1)/4 = 0.5, pow(0.5, 0.6) ≈ 0.66, base = 1-0.66 ≈ 0.34
	// Bonus: clamp01(12/15) * 0.15 ≈ 0.12
	// Total ≈ 0.46
	if got < 0.40 || got > 0.55 {
		t.Errorf("handScore(3, maxHand=4, 2014) = %.3f, want [0.40, 0.55]", got)
	}

	// Verify age bonus is applied: same hand/maxHand with a new car should score lower.
	newCar := handScore(3, 4, 2024)
	if got <= newCar {
		t.Errorf("hand 3 on 2014 (%.3f) should beat hand 3 on 2024 (%.3f)", got, newCar)
	}
}

func TestYearScore_AtMax(t *testing.T) {
	score := yearScore(2024, 2018, 2024)
	if score != 1.0 {
		t.Errorf("yearScore at yearMax should be 1.0, got %.4f", score)
	}
}

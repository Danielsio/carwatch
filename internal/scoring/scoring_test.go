package scoring

import (
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
	median, cohort, ok := mc.Lookup("Toyota", "Corolla", 2020)
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
	_, _, ok := mc.Lookup("Toyota", "Corolla", 2020)
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
	_, cohort, ok := mc.Lookup("Honda", "Civic", 2020)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cohort != 12 {
		t.Errorf("expected cohort=12, got %d", cohort)
	}

	// Looking up year 2022 should only include 2021 (year+-1)
	_, _, ok = mc.Lookup("Honda", "Civic", 2022)
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
	_, _, ok := mc.Lookup("toyota", "corolla", 2020)
	if !ok {
		t.Error("expected case-insensitive lookup to work")
	}
}

func TestMarketCache_Empty(t *testing.T) {
	mc := NewMarketCache(nil)
	_, _, ok := mc.Lookup("Toyota", "Corolla", 2020)
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
	median, _, ok := mc.Lookup("Mazda", "3", 2021)
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
			name: "unknown km gets neutral score",
			p: FitnessParams{
				Price: 150000, Km: 0, Hand: 1, Year: 2024, EngineVolume: 2000,
				PriceMax: 200000, MaxKm: 100000, MaxHand: 3, YearMin: 2020, YearMax: 2024, EngineMinCC: 1500,
			},
			min: 5.0, max: 8.0,
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

func TestKmScore_UnknownIsNeutral(t *testing.T) {
	got := kmScore(0, 150000)
	if got != 0.5 {
		t.Errorf("kmScore(0, 150000) = %.2f, want 0.5 (neutral for unknown)", got)
	}
}

func TestKmScore_NegativeIsNeutral(t *testing.T) {
	got := kmScore(-1, 150000)
	if got != 0.5 {
		t.Errorf("kmScore(-1, 150000) = %.2f, want 0.5 (neutral for unknown)", got)
	}
}

func TestKmScore_KnownLowBeatsUnknown(t *testing.T) {
	low := kmScore(10000, 150000)
	unknown := kmScore(0, 150000)
	if low <= unknown {
		t.Errorf("low-km listing (%.3f) should score higher than unknown-km (%.3f)", low, unknown)
	}
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

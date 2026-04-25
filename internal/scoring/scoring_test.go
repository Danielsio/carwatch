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

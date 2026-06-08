package model

import (
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestSourceParamsFromSearch(t *testing.T) {
	s := &storage.Search{
		Manufacturer: 27,
		Model:        10332,
		YearMin:      2020,
		YearMax:      2024,
		PriceMin:     50000,
		PriceMax:     150000,
		MaxKm:        130000,
		MaxHand:      3,
		EngineMinCC:  1600,
	}

	p := SourceParamsFromSearch(s)

	if p.Manufacturer != 27 {
		t.Errorf("Manufacturer = %d, want 27", p.Manufacturer)
	}
	if p.Model != 10332 {
		t.Errorf("Model = %d, want 10332", p.Model)
	}
	if p.YearMin != 2020 {
		t.Errorf("YearMin = %d, want 2020", p.YearMin)
	}
	if p.YearMax != 2024 {
		t.Errorf("YearMax = %d, want 2024", p.YearMax)
	}
	if p.PriceMin != 50000 {
		t.Errorf("PriceMin = %d, want 50000", p.PriceMin)
	}
	if p.PriceMax != 150000 {
		t.Errorf("PriceMax = %d, want 150000", p.PriceMax)
	}
	if p.MaxKm != 130000 {
		t.Errorf("MaxKm = %d, want 130000", p.MaxKm)
	}
	if p.MaxHand != 3 {
		t.Errorf("MaxHand = %d, want 3", p.MaxHand)
	}
	if p.EngineMinCC != 1600 {
		t.Errorf("EngineMinCC = %d, want 1600", p.EngineMinCC)
	}
	if p.Page != 0 {
		t.Errorf("Page = %d, want 0 (not set by SourceParamsFromSearch)", p.Page)
	}
}

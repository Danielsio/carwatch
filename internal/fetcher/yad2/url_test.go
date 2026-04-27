package yad2

import (
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/model"
)

func TestBuildURL_Basic(t *testing.T) {
	params := model.SourceParams{}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "Order=1") {
		t.Errorf("URL should contain Order=1, got %q", u)
	}
}

func TestBuildURL_WithManufacturer(t *testing.T) {
	params := model.SourceParams{Manufacturer: 19}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "manufacturer=19") {
		t.Errorf("URL should contain manufacturer=19, got %q", u)
	}
}

func TestBuildURL_WithModel(t *testing.T) {
	params := model.SourceParams{Manufacturer: 19, Model: 10226}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "model=10226") {
		t.Errorf("URL should contain model=10226, got %q", u)
	}
}

func TestBuildURL_WithYearRange(t *testing.T) {
	params := model.SourceParams{YearMin: 2018, YearMax: 2024}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "year=2018-2024") {
		t.Errorf("URL should contain year=2018-2024, got %q", u)
	}
}

func TestBuildURL_WithYearMinOnly(t *testing.T) {
	params := model.SourceParams{YearMin: 2020}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "year=2020-2030") {
		t.Errorf("URL should default YearMax to 2030, got %q", u)
	}
}

func TestBuildURL_WithYearMaxOnly(t *testing.T) {
	params := model.SourceParams{YearMax: 2024}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "year=2000-2024") {
		t.Errorf("URL should default YearMin to 2000, got %q", u)
	}
}

func TestBuildURL_WithPriceRange(t *testing.T) {
	params := model.SourceParams{PriceMin: 50000, PriceMax: 200000}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "price=50000-200000") {
		t.Errorf("URL should contain price=50000-200000, got %q", u)
	}
}

func TestBuildURL_WithPriceMaxOnly(t *testing.T) {
	params := model.SourceParams{PriceMax: 150000}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "price=0-150000") {
		t.Errorf("URL should default PriceMin to 0, got %q", u)
	}
}

func TestBuildURL_WithPriceMinOnly(t *testing.T) {
	params := model.SourceParams{PriceMin: 50000}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "price=50000-9999999") {
		t.Errorf("URL should default PriceMax to 9999999, got %q", u)
	}
}

func TestBuildURL_WithPage(t *testing.T) {
	params := model.SourceParams{Page: 3}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if !strings.Contains(u, "page=3") {
		t.Errorf("URL should contain page=3, got %q", u)
	}
}

func TestBuildURL_NoPageWhenZero(t *testing.T) {
	params := model.SourceParams{}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if strings.Contains(u, "page=") {
		t.Errorf("URL should not contain page= when Page is 0, got %q", u)
	}
}

func TestBuildURL_FullParams(t *testing.T) {
	params := model.SourceParams{
		Manufacturer: 19,
		Model:        10226,
		YearMin:      2018,
		YearMax:      2024,
		PriceMax:     150000,
		Page:         1,
	}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)

	checks := []string{
		"manufacturer=19",
		"model=10226",
		"year=2018-2024",
		"price=0-150000",
		"Order=1",
		"page=1",
	}
	for _, check := range checks {
		if !strings.Contains(u, check) {
			t.Errorf("URL missing %q: %s", check, u)
		}
	}
}

func TestBuildURL_NoManufacturerOrModel(t *testing.T) {
	params := model.SourceParams{}
	u := buildURL("https://www.yad2.co.il/vehicles/cars", params)
	if strings.Contains(u, "manufacturer=") {
		t.Errorf("URL should not have manufacturer when 0, got %q", u)
	}
	if strings.Contains(u, "model=") {
		t.Errorf("URL should not have model when 0, got %q", u)
	}
}

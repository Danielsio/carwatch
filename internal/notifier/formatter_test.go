package notifier

import (
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func TestFormatListing(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token:        "abc123",
			Manufacturer: "Mazda",
			Model:        "3",
			Year:         2021,
			EngineVolume: 1998,
			GearBox:      "Automatic",
			Km:           85000,
			Hand:         2,
			City:         "Tel Aviv",
			Area:         "Center",
			Price:        95000,
			PageLink:     "https://www.yad2.co.il/item/abc123",
		},
	}

	msg := FormatListing(l, locale.English)

	checks := []string{
		"Mazda 3",
		"2021",
		"2.0L",
		"Automatic",
		"85,000",
		"Tel Aviv",
		"95,000",
		"https://www.yad2.co.il/item/abc123",
	}

	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Errorf("message missing %q:\n%s", check, msg)
		}
	}
}

func TestFormatPriceDrop(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token:        "abc123",
			Manufacturer: "Mazda",
			Model:        "3",
			Year:         2021,
			Price:        89000,
			Km:           85000,
			Hand:         2,
			PageLink:     "https://www.yad2.co.il/item/abc123",
		},
	}

	msg := FormatPriceDrop(l, 95000, locale.English)

	checks := []string{
		"Price Drop!",
		"Mazda 3 2021",
		"₪95,000",
		"₪89,000",
		"-₪6,000",
		"85,000 km",
		"Hand 2",
		"https://www.yad2.co.il/item/abc123",
	}

	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Errorf("message missing %q:\n%s", check, msg)
		}
	}
}

func TestFormatPriceDrop_MinimalFields(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token:        "xyz",
			Manufacturer: "Toyota",
			Model:        "Corolla",
			SubModel:     "GLi",
			Price:        70000,
		},
	}

	msg := FormatPriceDrop(l, 80000, locale.English)

	if !strings.Contains(msg, "Toyota Corolla GLi") {
		t.Errorf("should include submodel in title:\n%s", msg)
	}
	if !strings.Contains(msg, "-₪10,000") {
		t.Errorf("should show correct drop amount:\n%s", msg)
	}
	if strings.Contains(msg, "km") {
		t.Errorf("should not show mileage when zero:\n%s", msg)
	}
}


func TestFormatListing_EscapesMarkdown(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token:        "md1",
			Manufacturer: "Land_Rover",
			Model:        "Range*Rover",
			SubModel:     "Sport`Ed",
			Year:         2022,
			City:         "Tel_Aviv",
			Area:         "Center[South]",
			GearBox:      "Auto_matic",
			EngineVolume: 3000,
			Price:        300000,
			PageLink:     "https://example.com/item_123",
		},
	}

	msg := FormatListing(l, locale.English)

	if strings.Contains(msg, "Land_Rover") {
		t.Error("underscore in manufacturer should be escaped")
	}
	if !strings.Contains(msg, "Land\\_Rover") {
		t.Errorf("expected escaped manufacturer, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Range\\*Rover") {
		t.Errorf("expected escaped model, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Sport\\`Ed") {
		t.Errorf("expected escaped submodel, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Tel\\_Aviv") {
		t.Errorf("expected escaped city, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Center\\[South\\]") {
		t.Errorf("expected escaped area, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Auto\\_matic") {
		t.Errorf("expected escaped gearbox, got:\n%s", msg)
	}
	if !strings.Contains(msg, "item\\_123") {
		t.Errorf("expected escaped page link, got:\n%s", msg)
	}
}

func TestFormatPriceDrop_EscapesMarkdown(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token:        "md2",
			Manufacturer: "Land_Rover",
			Model:        "Range*Rover",
			Year:         2022,
			Price:        280000,
			PageLink:     "https://example.com/item_456",
		},
	}

	msg := FormatPriceDrop(l, 300000, locale.English)

	if !strings.Contains(msg, "Land\\_Rover") {
		t.Errorf("expected escaped manufacturer in price drop, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Range\\*Rover") {
		t.Errorf("expected escaped model in price drop, got:\n%s", msg)
	}
	if !strings.Contains(msg, "item\\_456") {
		t.Errorf("expected escaped page link in price drop, got:\n%s", msg)
	}
}

func TestFormatBatch_SingleListing(t *testing.T) {
	listings := []model.Listing{{
		RawListing: model.RawListing{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 90000},
	}}

	msg := FormatBatch(listings, locale.English)
	if strings.Contains(msg, "2 New Listings") {
		t.Error("single listing should not use batch header")
	}
}

func TestFormatBatch_MultipleListing(t *testing.T) {
	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Mazda", Model: "3"}},
		{RawListing: model.RawListing{Token: "b", Manufacturer: "Mazda", Model: "3"}},
	}

	msg := FormatBatch(listings, locale.English)
	if !strings.Contains(msg, "2 New Listings") {
		t.Error("batch should contain count header")
	}
	if !strings.Contains(msg, "[1/2]") || !strings.Contains(msg, "[2/2]") {
		t.Error("batch should contain numbered entries")
	}
}

func TestFormatListing_WithScore(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token: "s1", Manufacturer: "Toyota", Model: "Corolla",
			Year: 2022, Price: 80000,
		},
		DealScore: &model.ScoreInfo{Score: 20, MedianPrice: 100000, CohortSize: 15},
	}

	msg := FormatListing(l, locale.English)

	checks := []string{
		"Deal Score: 20/100",
		"below market",
		"100,000",
	}
	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Errorf("message missing %q:\n%s", check, msg)
		}
	}
}

func TestFormatListing_WithoutScore_NoChange(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token: "ns1", Manufacturer: "Mazda", Model: "3",
			Year: 2021, Price: 95000,
		},
	}

	msg := FormatListing(l, locale.English)
	if strings.Contains(msg, "Deal Score") {
		t.Errorf("should not show score when DealScore is nil:\n%s", msg)
	}
}

func TestFormatListing_ScoreAtMedian(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token: "s2", Manufacturer: "Honda", Model: "Civic",
			Year: 2020, Price: 100000,
		},
		DealScore: &model.ScoreInfo{Score: 0, MedianPrice: 100000, CohortSize: 12},
	}

	msg := FormatListing(l, locale.English)
	if !strings.Contains(msg, "Deal Score: 0/100") {
		t.Errorf("expected score 0:\n%s", msg)
	}
	if !strings.Contains(msg, "Near market price") {
		t.Errorf("expected near market text:\n%s", msg)
	}
}

func TestFormatListing_ScoreAboveMedian(t *testing.T) {
	l := model.Listing{
		RawListing: model.RawListing{
			Token: "s3", Manufacturer: "BMW", Model: "3",
			Year: 2019, Price: 150000,
		},
		DealScore: &model.ScoreInfo{Score: 0, MedianPrice: 100000, CohortSize: 20},
	}

	msg := FormatListing(l, locale.English)
	if !strings.Contains(msg, "Above market price") {
		t.Errorf("expected above market text:\n%s", msg)
	}
}

func TestFormatDailyDigest(t *testing.T) {
	stats := []storage.DailySearchStats{
		{
			SearchName:    "toyota-corolla",
			NewCount:      7,
			AvgPrice:      165000,
			BestPrice:     148000,
			BestPriceLink: "https://www.yad2.co.il/item/abc",
			PriceTrend:    -2.5,
		},
		{
			SearchName: "honda-civic",
			NewCount:   3,
			AvgPrice:   120000,
			BestPrice:  110000,
			PriceTrend: 0.5,
		},
	}

	msg := FormatDailyDigest(stats, locale.English)

	checks := []string{
		"Daily Market Summary",
		"toyota-corolla",
		"New (24h): 7",
		"165,000",
		"148,000",
		"yad2.co.il",
		"Trending down 2.5%",
		"honda-civic",
		"Prices stable",
	}
	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Errorf("digest missing %q:\n%s", check, msg)
		}
	}
}

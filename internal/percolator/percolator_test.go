package percolator

import (
	"testing"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func boolPtr(v bool) *bool { return &v }

func TestMatch_BasicManufacturerAndModel(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Name: "mazda3", Manufacturer: 27, Model: 10332, Active: true},
	})

	// Should match.
	matches := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
	})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SearchID != 1 {
		t.Errorf("expected search ID 1, got %d", matches[0].SearchID)
	}
	if matches[0].ChatID != 100 {
		t.Errorf("expected chat ID 100, got %d", matches[0].ChatID)
	}

	// Wrong model - should not match.
	matches = p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 99999, Year: 2020, Price: 100000,
	})
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for wrong model, got %d", len(matches))
	}

	// Wrong manufacturer - should not match.
	matches = p.Match(model.RawListing{
		Token: "c", ManufacturerID: 19, ModelID: 10332, Year: 2020, Price: 100000,
	})
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for wrong manufacturer, got %d", len(matches))
	}
}

func TestMatch_YearRange(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			YearMin: 2018, YearMax: 2022, Active: true},
	})

	tests := []struct {
		year    int
		wantN   int
		comment string
	}{
		{2017, 0, "below min year"},
		{2018, 1, "at min year"},
		{2020, 1, "within range"},
		{2022, 1, "at max year"},
		{2023, 0, "above max year"},
	}

	for _, tt := range tests {
		matches := p.Match(model.RawListing{
			Token: "t", ManufacturerID: 27, ModelID: 10332, Year: tt.year, Price: 100000,
		})
		if len(matches) != tt.wantN {
			t.Errorf("year=%d (%s): got %d matches, want %d",
				tt.year, tt.comment, len(matches), tt.wantN)
		}
	}
}

func TestMatch_PriceRange(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			PriceMin: 50000, PriceMax: 150000, Active: true},
	})

	tests := []struct {
		price   int
		wantN   int
		comment string
	}{
		{49999, 0, "below min price"},
		{50000, 1, "at min price"},
		{100000, 1, "within range"},
		{150000, 1, "at max price"},
		{150001, 0, "above max price"},
		{0, 1, "zero price (unknown) passes PriceMin check"},
	}

	for _, tt := range tests {
		matches := p.Match(model.RawListing{
			Token: "t", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: tt.price,
		})
		if len(matches) != tt.wantN {
			t.Errorf("price=%d (%s): got %d matches, want %d",
				tt.price, tt.comment, len(matches), tt.wantN)
		}
	}
}

func TestMatch_KmAndHand(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			MaxKm: 100000, MaxHand: 3, Active: true},
	})

	// Within limits.
	if m := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Km: 80000, Hand: 2,
	}); len(m) != 1 {
		t.Errorf("within limits: expected 1, got %d", len(m))
	}

	// Km too high.
	if m := p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Km: 100001, Hand: 2,
	}); len(m) != 0 {
		t.Errorf("km too high: expected 0, got %d", len(m))
	}

	// Hand too high.
	if m := p.Match(model.RawListing{
		Token: "c", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Km: 50000, Hand: 4,
	}); len(m) != 0 {
		t.Errorf("hand too high: expected 0, got %d", len(m))
	}

	// Unknown km (0) should pass.
	if m := p.Match(model.RawListing{
		Token: "d", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Km: 0, Hand: 2,
	}); len(m) != 1 {
		t.Errorf("unknown km: expected 1, got %d", len(m))
	}
}

func TestMatch_EngineMinCC(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			EngineMinCC: 1800, Active: true},
	})

	// Engine too small.
	if m := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		EngineVolume: 1500,
	}); len(m) != 0 {
		t.Errorf("engine too small: expected 0, got %d", len(m))
	}

	// Engine big enough.
	if m := p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		EngineVolume: 2000,
	}); len(m) != 1 {
		t.Errorf("engine big enough: expected 1, got %d", len(m))
	}

	// Unknown engine (0) should be rejected when EngineMinCC is set
	// (consistent with filter.Apply behavior).
	if m := p.Match(model.RawListing{
		Token: "c", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		EngineVolume: 0,
	}); len(m) != 0 {
		t.Errorf("unknown engine: expected 0, got %d", len(m))
	}
}

func TestMatch_GearBox(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			GearBox: "automatic", Active: true},
	})

	// Matching gearbox (case-insensitive).
	if m := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		GearBox: "Automatic",
	}); len(m) != 1 {
		t.Errorf("matching gearbox: expected 1, got %d", len(m))
	}

	// Wrong gearbox.
	if m := p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		GearBox: "manual",
	}); len(m) != 0 {
		t.Errorf("wrong gearbox: expected 0, got %d", len(m))
	}

	// Unknown gearbox (empty) should pass.
	if m := p.Match(model.RawListing{
		Token: "c", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		GearBox: "",
	}); len(m) != 1 {
		t.Errorf("unknown gearbox: expected 1, got %d", len(m))
	}
}

func TestMatch_SellerFilter(t *testing.T) {
	tests := []struct {
		name       string
		filter     string
		commercial *bool
		wantMatch  bool
	}{
		{"private filter, private seller", "private", boolPtr(false), true},
		{"private filter, commercial seller", "private", boolPtr(true), false},
		{"commercial filter, commercial seller", "commercial", boolPtr(true), true},
		{"commercial filter, private seller", "commercial", boolPtr(false), false},
		{"dealer filter, commercial seller", "dealer", boolPtr(true), true},
		{"dealer filter, private seller", "dealer", boolPtr(false), false},
		{"any filter, commercial", "any", boolPtr(true), true},
		{"any filter, private", "any", boolPtr(false), true},
		{"empty filter, commercial", "", boolPtr(true), true},
		{"private filter, unknown seller", "private", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			p.Load([]storage.Search{
				{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
					SellerFilter: tt.filter, Active: true},
			})
			m := p.Match(model.RawListing{
				Token: "t", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
				Commercial: tt.commercial,
			})
			got := len(m) > 0
			if got != tt.wantMatch {
				t.Errorf("got match=%v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestMatch_PriceOnly(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			PriceOnly: true, Active: true},
	})

	// Has price.
	if m := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
	}); len(m) != 1 {
		t.Errorf("has price: expected 1, got %d", len(m))
	}

	// No price.
	if m := p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 0,
	}); len(m) != 0 {
		t.Errorf("no price: expected 0, got %d", len(m))
	}
}

func TestMatch_PhotoOnly(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			PhotoOnly: true, Active: true},
	})

	// Has photo.
	if m := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		ImageURL: "https://img.com/1.jpg",
	}); len(m) != 1 {
		t.Errorf("has photo: expected 1, got %d", len(m))
	}

	// No photo.
	if m := p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		ImageURL: "",
	}); len(m) != 0 {
		t.Errorf("no photo: expected 0, got %d", len(m))
	}
}

func TestMatch_Keywords(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			Keywords: "hybrid, sunroof", Active: true},
	})

	// Both keywords present.
	if m := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Description: "Great hybrid car with sunroof",
	}); len(m) != 1 {
		t.Errorf("both keywords: expected 1, got %d", len(m))
	}

	// Only one keyword present.
	if m := p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Description: "Great hybrid car",
	}); len(m) != 0 {
		t.Errorf("one keyword: expected 0, got %d", len(m))
	}

	// Keywords in SubModel.
	if m := p.Match(model.RawListing{
		Token: "c", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Description: "sunroof model", SubModel: "Hybrid Premium",
	}); len(m) != 1 {
		t.Errorf("keyword in submodel: expected 1, got %d", len(m))
	}
}

func TestMatch_ExcludeKeywords(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			ExcludeKeys: "accident, salvage", Active: true},
	})

	// No excluded keywords.
	if m := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Description: "Perfect condition",
	}); len(m) != 1 {
		t.Errorf("no excluded keywords: expected 1, got %d", len(m))
	}

	// Has excluded keyword.
	if m := p.Match(model.RawListing{
		Token: "b", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Description: "Minor accident damage",
	}); len(m) != 0 {
		t.Errorf("has excluded keyword: expected 0, got %d", len(m))
	}

	// Excluded keyword in SubModel.
	if m := p.Match(model.RawListing{
		Token: "c", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
		Description: "Good car", SubModel: "Salvage Title",
	}); len(m) != 0 {
		t.Errorf("excluded in submodel: expected 0, got %d", len(m))
	}
}

func TestMatch_NoMatch(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			YearMin: 2020, PriceMax: 100000, Active: true},
	})

	// Listing doesn't match any search (wrong manufacturer, wrong model, etc.).
	matches := p.Match(model.RawListing{
		Token: "x", ManufacturerID: 19, ModelID: 5000, Year: 2015, Price: 200000,
	})
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestMatch_MultipleSearchesMatch(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Name: "user1-mazda3", Manufacturer: 27, Model: 10332,
			YearMin: 2018, PriceMax: 150000, Active: true},
		{ID: 2, ChatID: 200, Name: "user2-mazda3", Manufacturer: 27, Model: 10332,
			YearMin: 2019, PriceMax: 120000, Active: true},
		{ID: 3, ChatID: 300, Name: "user3-toyota", Manufacturer: 19, Model: 1,
			YearMin: 2018, PriceMax: 150000, Active: true},
	})

	listing := model.RawListing{
		Token: "tok1", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
	}

	matches := p.Match(listing)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches (user1 and user2), got %d", len(matches))
	}

	chatIDs := map[int64]bool{}
	for _, m := range matches {
		chatIDs[m.ChatID] = true
	}
	if !chatIDs[100] || !chatIDs[200] {
		t.Errorf("expected chatIDs 100 and 200, got %v", chatIDs)
	}
}

func TestMatch_EmptyPercolator(t *testing.T) {
	p := New()
	matches := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
	})
	if len(matches) != 0 {
		t.Errorf("empty percolator: expected 0, got %d", len(matches))
	}
}

func TestMatch_WildcardManufacturer(t *testing.T) {
	// Search with Manufacturer=0 should match any manufacturer.
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 0, Model: 0,
			YearMin: 2020, PriceMax: 100000, Active: true},
	})

	matches := p.Match(model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2021, Price: 90000,
	})
	if len(matches) != 1 {
		t.Errorf("wildcard manufacturer: expected 1, got %d", len(matches))
	}
}

func TestLoad_OverwritesPreviousRules(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332, Active: true},
	})

	listing := model.RawListing{
		Token: "a", ManufacturerID: 27, ModelID: 10332, Year: 2020, Price: 100000,
	}

	if m := p.Match(listing); len(m) != 1 {
		t.Fatalf("before reload: expected 1, got %d", len(m))
	}

	// Replace with different search.
	p.Load([]storage.Search{
		{ID: 2, ChatID: 200, Manufacturer: 19, Model: 1, Active: true},
	})

	if m := p.Match(listing); len(m) != 0 {
		t.Errorf("after reload: expected 0 (different mfr), got %d", len(m))
	}
}

func TestMatch_CombinedFilters(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332,
			YearMin: 2019, YearMax: 2023,
			PriceMin: 80000, PriceMax: 130000,
			MaxKm: 80000, MaxHand: 2,
			EngineMinCC: 1800,
			GearBox:     "automatic",
			PriceOnly:   true,
			PhotoOnly:   true,
			Keywords:    "hybrid",
			ExcludeKeys: "accident",
			Active:      true,
		},
	})

	// Perfect match.
	if m := p.Match(model.RawListing{
		Token: "perfect", ManufacturerID: 27, ModelID: 10332,
		Year: 2021, Price: 100000, Km: 50000, Hand: 1,
		EngineVolume: 2000, GearBox: "Automatic",
		ImageURL:    "https://img.com/1.jpg",
		Description: "Hybrid edition, great condition",
	}); len(m) != 1 {
		t.Errorf("perfect match: expected 1, got %d", len(m))
	}

	// Fails on year.
	if m := p.Match(model.RawListing{
		Token: "bad-year", ManufacturerID: 27, ModelID: 10332,
		Year: 2018, Price: 100000, Km: 50000, Hand: 1,
		EngineVolume: 2000, GearBox: "Automatic",
		ImageURL:    "https://img.com/1.jpg",
		Description: "Hybrid edition",
	}); len(m) != 0 {
		t.Errorf("bad year: expected 0, got %d", len(m))
	}

	// Fails on excluded keyword.
	if m := p.Match(model.RawListing{
		Token: "bad-kw", ManufacturerID: 27, ModelID: 10332,
		Year: 2021, Price: 100000, Km: 50000, Hand: 1,
		EngineVolume: 2000, GearBox: "Automatic",
		ImageURL:    "https://img.com/1.jpg",
		Description: "Hybrid edition, minor accident",
	}); len(m) != 0 {
		t.Errorf("excluded keyword: expected 0, got %d", len(m))
	}
}

func TestClassifyRejection_PrimaryReason(t *testing.T) {
	s := storage.Search{
		ID: 1, Manufacturer: 27, Model: 10332,
		YearMin: 2016, YearMax: 2021, PriceMax: 85000, MaxKm: 200000, MaxHand: 3,
	}

	tests := []struct {
		name   string
		l      model.RawListing
		expect RejectReason
	}{
		{"match", model.RawListing{ManufacturerID: 27, ModelID: 10332, Year: 2019, Price: 70000}, ""},
		{"wrong manufacturer", model.RawListing{ManufacturerID: 99, ModelID: 10332, Year: 2019, Price: 70000}, RejectWrongModel},
		{"wrong model", model.RawListing{ManufacturerID: 27, ModelID: 999, Year: 2019, Price: 70000}, RejectWrongModel},
		{"year too old", model.RawListing{ManufacturerID: 27, ModelID: 10332, Year: 2014, Price: 70000}, RejectYearOut},
		{"year too new", model.RawListing{ManufacturerID: 27, ModelID: 10332, Year: 2025, Price: 70000}, RejectYearOut},
		{"price over", model.RawListing{ManufacturerID: 27, ModelID: 10332, Year: 2019, Price: 90000}, RejectPriceOut},
		{"km over", model.RawListing{ManufacturerID: 27, ModelID: 10332, Year: 2019, Price: 70000, Km: 250000}, RejectKmOver},
		{"hand over", model.RawListing{ManufacturerID: 27, ModelID: 10332, Year: 2019, Price: 70000, Hand: 5}, RejectHandOver},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyRejection(tt.l, s)
			if got != tt.expect {
				t.Errorf("got %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestCountRejections_Integration(t *testing.T) {
	p := New()
	p.Load([]storage.Search{
		{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332, YearMin: 2016, YearMax: 2021, PriceMax: 85000},
	})

	listings := []model.RawListing{
		{ManufacturerID: 27, ModelID: 10332, Year: 2019, Price: 70000},  // match
		{ManufacturerID: 99, ModelID: 999, Year: 2019, Price: 70000},    // wrong_model
		{ManufacturerID: 27, ModelID: 10332, Year: 2014, Price: 70000},  // year_out
		{ManufacturerID: 27, ModelID: 10332, Year: 2019, Price: 100000}, // price_out
		{ManufacturerID: 19, ModelID: 10378, Year: 2020, Price: 50000},  // wrong_model
	}

	result := p.CountRejections(listings)
	counts := result[1]
	if counts[RejectWrongModel] != 2 {
		t.Errorf("wrong_model: got %d, want 2", counts[RejectWrongModel])
	}
	if counts[RejectYearOut] != 1 {
		t.Errorf("year_out: got %d, want 1", counts[RejectYearOut])
	}
	if counts[RejectPriceOut] != 1 {
		t.Errorf("price_out: got %d, want 1", counts[RejectPriceOut])
	}
	total := counts[RejectWrongModel] + counts[RejectYearOut] + counts[RejectPriceOut] +
		counts[RejectKmOver] + counts[RejectHandOver] + counts[RejectOtherFilter]
	if total != 4 {
		t.Errorf("total rejections: got %d, want 4 (1 match + 4 rejections = 5 listings)", total)
	}
}

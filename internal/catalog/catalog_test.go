package catalog

import (
	"testing"
)

func TestStaticCatalog_Manufacturers(t *testing.T) {
	cat := NewStatic()
	mfrs := cat.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected at least 10 manufacturers, got %d", len(mfrs))
	}
}

func TestStaticCatalog_Models(t *testing.T) {
	cat := NewStatic()
	models := cat.Models(27) // Mazda
	if len(models) == 0 {
		t.Error("expected Mazda to have models")
	}
	found := false
	for _, m := range models {
		if m.Name == "3" {
			found = true
		}
	}
	if !found {
		t.Error("Mazda 3 not found")
	}
}

func TestStaticCatalog_AllManufacturersHaveModels(t *testing.T) {
	cat := NewStatic()
	for _, m := range cat.Manufacturers() {
		if len(cat.Models(m.ID)) == 0 {
			t.Errorf("manufacturer %q (ID=%d) has no models", m.Name, m.ID)
		}
	}
}

func TestStaticCatalog_NameLookups(t *testing.T) {
	cat := NewStatic()
	if name := cat.ManufacturerName(27); name != "Mazda" {
		t.Errorf("ManufacturerName(27) = %q, want Mazda", name)
	}
	if name := cat.ManufacturerName(99999); name != "Unknown" {
		t.Errorf("ManufacturerName(99999) = %q, want Unknown", name)
	}
	if name := cat.ModelName(27, 10332); name != "3" {
		t.Errorf("ModelName(27, 10332) = %q, want 3", name)
	}
	if name := cat.ModelName(27, 99999); name != "Unknown" {
		t.Errorf("ModelName(27, 99999) = %q, want Unknown", name)
	}
}

func TestDynamicCatalog_FallsBackToStatic(t *testing.T) {
	cat := NewDynamic(nil, nil, nil)
	cat.mu.Lock()
	cat.mfrs = NewStatic().Manufacturers()
	for _, m := range cat.mfrs {
		cat.models[m.ID] = NewStatic().Models(m.ID)
	}
	cat.mu.Unlock()

	mfrs := cat.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected at least 10 manufacturers, got %d", len(mfrs))
	}

	if name := cat.ManufacturerName(27); name != "Mazda" {
		t.Errorf("ManufacturerName(27) = %q, want Mazda", name)
	}
}

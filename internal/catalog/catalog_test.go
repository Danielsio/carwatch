package catalog

import (
	"context"
	"log/slog"
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

func TestDynamicCatalog_LoadsFallback(t *testing.T) {
	cat := NewDynamic(nil, slog.Default())
	cat.Load(context.Background())

	mfrs := cat.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected at least 10 manufacturers, got %d", len(mfrs))
	}

	if name := cat.ManufacturerName(27); name != "Mazda" {
		t.Errorf("ManufacturerName(27) = %q, want Mazda", name)
	}
}

func TestDynamicCatalog_Ingest(t *testing.T) {
	cat := NewDynamic(nil, slog.Default())
	cat.Load(context.Background())

	before := len(cat.Manufacturers())
	ctx := context.Background()

	cat.Ingest(ctx, 999, "NewBrand", 88888, "NewModel")

	after := len(cat.Manufacturers())
	if after != before+1 {
		t.Errorf("expected %d manufacturers after ingest, got %d", before+1, after)
	}

	if name := cat.ManufacturerName(999); name != "NewBrand" {
		t.Errorf("ManufacturerName(999) = %q, want NewBrand", name)
	}

	models := cat.Models(999)
	if len(models) != 1 || models[0].Name != "NewModel" {
		t.Errorf("expected 1 model NewModel, got %v", models)
	}

	// Ingesting same entry again should not duplicate
	cat.Ingest(ctx, 999, "NewBrand", 88888, "NewModel")
	if len(cat.Models(999)) != 1 {
		t.Error("duplicate ingest should not add new entry")
	}
}

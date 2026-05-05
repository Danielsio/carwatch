package catalog

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestNewDynamic(t *testing.T) {
	d := NewDynamic(testLogger)

	if d.models == nil {
		t.Error("models map should be initialized")
	}
	if d.fallback == nil {
		t.Error("fallback should be set")
	}
}

func TestDynamicCatalog_Load_UsesFallback(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())

	mfrs := d.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected static fallback manufacturers (>10), got %d", len(mfrs))
	}
}

func TestDynamicCatalog_Ingest_NewManufacturerAndModel(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())

	before := len(d.Manufacturers())
	ctx := context.Background()

	d.Ingest(ctx, IngestEntry{ManufacturerID: 999, ManufacturerName: "NewBrand", ModelID: 88888, ModelName: "NewModel"})

	after := len(d.Manufacturers())
	if after != before+1 {
		t.Errorf("expected %d manufacturers after ingest, got %d", before+1, after)
	}

	if name := d.ManufacturerName(999); name != "NewBrand" {
		t.Errorf("ManufacturerName(999) = %q, want NewBrand", name)
	}

	models := d.Models(999)
	if len(models) != 1 || models[0].Name != "NewModel" {
		t.Errorf("expected 1 model NewModel, got %v", models)
	}
}

func TestDynamicCatalog_Ingest_DuplicateIgnored(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	ctx := context.Background()

	d.Ingest(ctx, IngestEntry{ManufacturerID: 999, ManufacturerName: "NewBrand", ModelID: 88888, ModelName: "NewModel"})
	d.Ingest(ctx, IngestEntry{ManufacturerID: 999, ManufacturerName: "NewBrand", ModelID: 88888, ModelName: "NewModel"})

	if len(d.Models(999)) != 1 {
		t.Error("duplicate ingest should not add new entry")
	}
}

func TestDynamicCatalog_Ingest_SkipsInvalid(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	ctx := context.Background()
	before := len(d.Manufacturers())

	d.Ingest(ctx, IngestEntry{ManufacturerID: 0, ManufacturerName: "", ModelID: 1, ModelName: "X"})
	d.Ingest(ctx, IngestEntry{ManufacturerID: 0, ManufacturerName: "Empty", ModelID: 1, ModelName: "X"})
	d.Ingest(ctx, IngestEntry{ManufacturerID: 5, ManufacturerName: "", ModelID: 1, ModelName: "X"})

	after := len(d.Manufacturers())
	if after != before {
		t.Errorf("invalid ingests should not change manufacturer count, was %d now %d", before, after)
	}
}

func TestDynamicCatalog_Ingest_ManufacturerWithoutModel(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	ctx := context.Background()
	before := len(d.Manufacturers())

	d.Ingest(ctx, IngestEntry{ManufacturerID: 999, ManufacturerName: "NoModelBrand"})

	after := len(d.Manufacturers())
	if after != before+1 {
		t.Errorf("manufacturer-only ingest should add manufacturer, was %d now %d", before, after)
	}
	if name := d.ManufacturerName(999); name != "NoModelBrand" {
		t.Errorf("ManufacturerName(999) = %q, want NoModelBrand", name)
	}
	if len(d.Models(999)) != 0 {
		t.Error("manufacturer-only ingest should not add models")
	}
}

func TestDynamicCatalog_Ingest_PreservesExistingName(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	ctx := context.Background()

	d.Ingest(ctx, IngestEntry{ManufacturerID: 19, ManufacturerName: "טויוטה", ModelID: 10226, ModelName: "קורולה"})

	if name := d.ManufacturerName(19); name != "Toyota" {
		t.Errorf("static name should be preserved, got %q", name)
	}
}

func TestDynamicCatalog_Flush_IsNoop(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())

	d.Ingest(context.Background(), IngestEntry{ManufacturerID: 999, ManufacturerName: "NewBrand", ModelID: 88888, ModelName: "NewModel"})
	d.Flush(context.Background())

	if name := d.ManufacturerName(999); name != "NewBrand" {
		t.Error("in-memory catalog should be intact after flush")
	}
}

func TestDynamicCatalog_Models_UnknownManufacturer(t *testing.T) {
	d := NewDynamic(testLogger)
	models := d.Models(99999)
	if models != nil {
		t.Errorf("expected nil for unknown manufacturer, got %v", models)
	}
}

func TestDynamicCatalog_ModelName(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	ctx := context.Background()

	d.Ingest(ctx, IngestEntry{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 100, ModelName: "A3"})

	if name := d.ModelName(9000, 100); name != "A3" {
		t.Errorf("ModelName(9000,100) = %q, want A3", name)
	}
	if name := d.ModelName(9000, 999); name != "Unknown" {
		t.Errorf("ModelName(9000,999) = %q, want Unknown", name)
	}
	if name := d.ModelName(999, 1); name != "Unknown" {
		t.Errorf("ModelName(999,1) = %q, want Unknown", name)
	}
}

func TestDynamicCatalog_SearchManufacturers(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	d.Ingest(context.Background(), IngestEntry{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 100, ModelName: "A3"})

	results := d.SearchManufacturers("alpha")
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'alpha'")
	}
	found := false
	for _, r := range results {
		if r.ID == 9000 && r.Name == "AlphaCar" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SearchManufacturers should find AlphaCar")
	}

	noResults := d.SearchManufacturers("zzzznonexistent")
	if len(noResults) != 0 {
		t.Errorf("expected 0 results for nonexistent query, got %d", len(noResults))
	}
}

func TestDynamicCatalog_SearchModels(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	d.Ingest(context.Background(), IngestEntry{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 100, ModelName: "BetaModel"})
	d.Ingest(context.Background(), IngestEntry{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 101, ModelName: "GammaModel"})

	results := d.SearchModels(9000, "beta")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'beta', got %d", len(results))
	}
	if results[0].Name != "BetaModel" {
		t.Errorf("expected BetaModel, got %s", results[0].Name)
	}

	noResults := d.SearchModels(9000, "zzzznonexistent")
	if len(noResults) != 0 {
		t.Errorf("expected 0 results for nonexistent query, got %d", len(noResults))
	}

	noMfr := d.SearchModels(99999, "any")
	if len(noMfr) != 0 {
		t.Errorf("expected 0 results for unknown manufacturer, got %d", len(noMfr))
	}
}

func TestDynamicCatalog_Manufacturers_DefensiveCopy(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())

	mfrs1 := d.Manufacturers()
	mfrs2 := d.Manufacturers()

	if len(mfrs1) == 0 {
		t.Fatal("expected manufacturers")
	}
	mfrs1[0].Name = "MUTATED"
	if mfrs2[0].Name == "MUTATED" {
		t.Error("Manufacturers() should return defensive copy")
	}
}

func TestDynamicCatalog_Models_DefensiveCopy(t *testing.T) {
	d := NewDynamic(testLogger)
	d.Load(context.Background())
	d.Ingest(context.Background(), IngestEntry{ManufacturerID: 9000, ManufacturerName: "TestBrand", ModelID: 100, ModelName: "ModelA"})

	models1 := d.Models(9000)
	models2 := d.Models(9000)

	if len(models1) == 0 {
		t.Fatal("expected models")
	}
	models1[0].Name = "MUTATED"
	if models2[0].Name == "MUTATED" {
		t.Error("Models() should return defensive copy")
	}
}

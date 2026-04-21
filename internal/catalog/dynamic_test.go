package catalog

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// --- mock implementations ---

type mockCatalogStore struct {
	entries      []storage.CatalogEntry
	loadErr      error
	saveErr      error
	saveCalled   bool
	savedEntries []storage.CatalogEntry
}

func (m *mockCatalogStore) SaveCatalogEntries(_ context.Context, entries []storage.CatalogEntry) error {
	m.saveCalled = true
	m.savedEntries = entries
	return m.saveErr
}

func (m *mockCatalogStore) LoadCatalogEntries(_ context.Context) ([]storage.CatalogEntry, error) {
	return m.entries, m.loadErr
}

func (m *mockCatalogStore) CatalogAge(_ context.Context) (time.Duration, error) {
	return 0, nil
}

// --- tests ---

func TestNewDynamic(t *testing.T) {
	store := &mockCatalogStore{}
	d := NewDynamic(store, testLogger)

	if d.store != store {
		t.Error("store not set")
	}
	if d.models == nil {
		t.Error("models map should be initialized")
	}
	if d.fallback == nil {
		t.Error("fallback should be set")
	}
}

func TestDynamicCatalog_Load_FromCache(t *testing.T) {
	store := &mockCatalogStore{
		entries: []storage.CatalogEntry{
			{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 100, ModelName: "Alpha1"},
			{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 101, ModelName: "Alpha2"},
			{ManufacturerID: 9001, ManufacturerName: "BetaCar", ModelID: 200, ModelName: "Beta1"},
		},
	}

	d := NewDynamic(store, testLogger)
	d.Load(context.Background())

	if name := d.ManufacturerName(9000); name != "AlphaCar" {
		t.Errorf("ManufacturerName(9000) = %q, want AlphaCar", name)
	}

	models := d.Models(9000)
	if len(models) != 2 {
		t.Fatalf("expected 2 AlphaCar models, got %d", len(models))
	}
}

func TestDynamicCatalog_Load_NoCache_UsesFallback(t *testing.T) {
	store := &mockCatalogStore{
		loadErr: errors.New("no cache"),
	}

	d := NewDynamic(store, testLogger)
	d.Load(context.Background())

	mfrs := d.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected static fallback manufacturers (>10), got %d", len(mfrs))
	}
}

func TestDynamicCatalog_Load_NilStore_UsesFallback(t *testing.T) {
	d := NewDynamic(nil, testLogger)
	d.Load(context.Background())

	mfrs := d.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected static fallback manufacturers (>10), got %d", len(mfrs))
	}
}

func TestDynamicCatalog_Ingest_NewManufacturerAndModel(t *testing.T) {
	d := NewDynamic(nil, testLogger)
	d.Load(context.Background())

	before := len(d.Manufacturers())
	ctx := context.Background()

	d.Ingest(ctx, 999, "NewBrand", 88888, "NewModel")

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
	d := NewDynamic(nil, testLogger)
	d.Load(context.Background())
	ctx := context.Background()

	d.Ingest(ctx, 999, "NewBrand", 88888, "NewModel")
	d.Ingest(ctx, 999, "NewBrand", 88888, "NewModel")

	if len(d.Models(999)) != 1 {
		t.Error("duplicate ingest should not add new entry")
	}
}

func TestDynamicCatalog_Ingest_SkipsInvalid(t *testing.T) {
	d := NewDynamic(nil, testLogger)
	d.Load(context.Background())
	ctx := context.Background()
	before := len(d.Manufacturers())

	d.Ingest(ctx, 0, "", 1, "X")
	d.Ingest(ctx, 0, "Empty", 1, "X")
	d.Ingest(ctx, 5, "", 1, "X")

	after := len(d.Manufacturers())
	if after != before {
		t.Errorf("invalid ingests should not change manufacturer count, was %d now %d", before, after)
	}
}

func TestDynamicCatalog_Ingest_ManufacturerWithoutModel(t *testing.T) {
	d := NewDynamic(nil, testLogger)
	d.Load(context.Background())
	ctx := context.Background()
	before := len(d.Manufacturers())

	d.Ingest(ctx, 999, "NoModelBrand", 0, "")

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
	d := NewDynamic(nil, testLogger)
	d.Load(context.Background())
	ctx := context.Background()

	d.Ingest(ctx, 19, "טויוטה", 10226, "קורולה")

	if name := d.ManufacturerName(19); name != "Toyota" {
		t.Errorf("static name should be preserved, got %q", name)
	}
}

func TestDynamicCatalog_Flush_SavesWhenDirty(t *testing.T) {
	store := &mockCatalogStore{}
	d := NewDynamic(store, testLogger)
	d.Load(context.Background())

	d.Ingest(context.Background(), 999, "NewBrand", 88888, "NewModel")
	d.Flush(context.Background())

	if !store.saveCalled {
		t.Error("Flush should save when dirty")
	}
}

func TestDynamicCatalog_Flush_SaveError(t *testing.T) {
	store := &mockCatalogStore{saveErr: errors.New("db write failed")}
	d := NewDynamic(store, testLogger)
	d.Load(context.Background())

	d.Ingest(context.Background(), 999, "NewBrand", 88888, "NewModel")
	d.Flush(context.Background())

	if name := d.ManufacturerName(999); name != "NewBrand" {
		t.Error("in-memory catalog should be intact even on save error")
	}
}

func TestDynamicCatalog_Flush_PersistsManufacturersWithoutModels(t *testing.T) {
	store := &mockCatalogStore{}
	d := NewDynamic(store, testLogger)
	d.Load(context.Background())

	d.Ingest(context.Background(), 999, "NoModelBrand", 0, "")
	d.Flush(context.Background())

	if !store.saveCalled {
		t.Fatal("Flush should save")
	}

	found := false
	for _, e := range store.savedEntries {
		if e.ManufacturerID == 999 && e.ManufacturerName == "NoModelBrand" {
			found = true
		}
	}
	if !found {
		t.Error("manufacturers without models should be persisted")
	}
}

func TestDynamicCatalog_LoadFromStore(t *testing.T) {
	store := &mockCatalogStore{
		entries: []storage.CatalogEntry{
			{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 100, ModelName: "Alpha1"},
			{ManufacturerID: 9000, ManufacturerName: "AlphaCar", ModelID: 101, ModelName: "Alpha2"},
		},
	}

	d := NewDynamic(store, testLogger)
	ok := d.loadFromStore(context.Background())

	if !ok {
		t.Fatal("loadFromStore should return true")
	}
	if name := d.ManufacturerName(9000); name != "AlphaCar" {
		t.Errorf("ManufacturerName(9000) = %q, want AlphaCar", name)
	}
	if len(d.Models(9000)) != 2 {
		t.Error("models not loaded correctly")
	}
}

func TestDynamicCatalog_LoadFromStore_Error(t *testing.T) {
	store := &mockCatalogStore{loadErr: errors.New("db error")}
	d := NewDynamic(store, testLogger)
	if d.loadFromStore(context.Background()) {
		t.Error("loadFromStore should return false on error")
	}
}

func TestDynamicCatalog_LoadFromStore_Empty(t *testing.T) {
	store := &mockCatalogStore{entries: nil}
	d := NewDynamic(store, testLogger)
	if d.loadFromStore(context.Background()) {
		t.Error("loadFromStore should return false when empty")
	}
}

func TestDynamicCatalog_Models_UnknownManufacturer(t *testing.T) {
	d := NewDynamic(nil, testLogger)
	models := d.Models(99999)
	if models != nil {
		t.Errorf("expected nil for unknown manufacturer, got %v", models)
	}
}

func TestDynamicCatalog_ModelName(t *testing.T) {
	store := &mockCatalogStore{
		entries: []storage.CatalogEntry{
			{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
		},
	}
	d := NewDynamic(store, testLogger)
	d.loadFromStore(context.Background())

	if name := d.ModelName(1, 100); name != "A3" {
		t.Errorf("ModelName(1,100) = %q, want A3", name)
	}
	if name := d.ModelName(1, 999); name != "Unknown" {
		t.Errorf("ModelName(1,999) = %q, want Unknown", name)
	}
	if name := d.ModelName(999, 1); name != "Unknown" {
		t.Errorf("ModelName(999,1) = %q, want Unknown", name)
	}
}

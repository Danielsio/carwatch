package sqlite

import (
	"context"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestSaveCatalogEntries(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	entries := []storage.CatalogEntry{
		{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
		{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 101, ModelName: "A4"},
		{ManufacturerID: 7, ManufacturerName: "BMW", ModelID: 200, ModelName: "3 Series"},
	}

	if err := store.SaveCatalogEntries(ctx, entries); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.LoadCatalogEntries(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded))
	}
}

func TestSaveCatalogEntries_ReplacesExisting(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	initial := []storage.CatalogEntry{
		{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
		{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 101, ModelName: "A4"},
	}
	if err := store.SaveCatalogEntries(ctx, initial); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	replacement := []storage.CatalogEntry{
		{ManufacturerID: 7, ManufacturerName: "BMW", ModelID: 200, ModelName: "3 Series"},
	}
	if err := store.SaveCatalogEntries(ctx, replacement); err != nil {
		t.Fatalf("replacement save: %v", err)
	}

	loaded, err := store.LoadCatalogEntries(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry after replacement, got %d", len(loaded))
	}
	if loaded[0].ManufacturerName != "BMW" {
		t.Errorf("expected BMW, got %q", loaded[0].ManufacturerName)
	}
}

func TestSaveCatalogEntries_Empty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.SaveCatalogEntries(ctx, nil); err != nil {
		t.Fatalf("save empty: %v", err)
	}

	loaded, err := store.LoadCatalogEntries(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 entries, got %d", len(loaded))
	}
}

func TestLoadCatalogEntries_Empty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	loaded, err := store.LoadCatalogEntries(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 entries, got %d", len(loaded))
	}
}

func TestLoadCatalogEntries_SortedByName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	entries := []storage.CatalogEntry{
		{ManufacturerID: 7, ManufacturerName: "BMW", ModelID: 200, ModelName: "5 Series"},
		{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A4"},
		{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 101, ModelName: "A3"},
	}
	if err := store.SaveCatalogEntries(ctx, entries); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.LoadCatalogEntries(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded))
	}
	if loaded[0].ManufacturerName != "Audi" || loaded[0].ModelName != "A3" {
		t.Errorf("first entry should be Audi A3, got %s %s", loaded[0].ManufacturerName, loaded[0].ModelName)
	}
}

func TestCatalogAge_WithData(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	entries := []storage.CatalogEntry{
		{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
	}
	if err := store.SaveCatalogEntries(ctx, entries); err != nil {
		t.Fatalf("save: %v", err)
	}

	age, err := store.CatalogAge(ctx)
	if err != nil {
		t.Fatalf("age: %v", err)
	}
	if age < 0 || age > 5_000_000_000 { // 5 seconds
		t.Errorf("expected age < 5s, got %v", age)
	}
}

func TestCatalogAge_Empty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	age, err := store.CatalogAge(ctx)
	if err != nil {
		t.Fatalf("age: %v", err)
	}
	if age < 24*3600_000_000_000 { // should be max duration
		t.Errorf("expected very large age for empty cache, got %v", age)
	}
}

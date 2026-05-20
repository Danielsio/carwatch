package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/pgtest"
)

func TestPrefillFromDB(t *testing.T) {
	store := pgtest.NewStore(t)
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "tok-1", ChatID: 100, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		Km: 50000, City: "Tel Aviv", ImageURL: "https://img.com/1.jpg",
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	s, err := NewWithOptions(cfg, &mockFetcher{}, store, &mockNotifier{}, testLogger(), Options{
		ListingStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}

	listings := []model.RawListing{
		{Token: "tok-1", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000, Km: 0, City: "", ImageURL: ""},
		{Token: "tok-2", Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 90000, Km: 30000, City: "Haifa", ImageURL: "https://img.com/2.jpg"},
	}

	s.prefillFromDB(ctx, listings)

	if listings[0].Km != 50000 {
		t.Errorf("tok-1 Km = %d, want 50000", listings[0].Km)
	}
	if listings[0].City != "Tel Aviv" {
		t.Errorf("tok-1 City = %q, want Tel Aviv", listings[0].City)
	}
	if listings[0].ImageURL != "https://img.com/1.jpg" {
		t.Errorf("tok-1 ImageURL = %q, want https://img.com/1.jpg", listings[0].ImageURL)
	}
	if listings[1].Km != 30000 {
		t.Errorf("tok-2 Km should remain 30000, got %d", listings[1].Km)
	}
}

func TestPrefillFromDB_AllFieldsPresent(t *testing.T) {
	cfg := testConfig()
	s, err := NewWithOptions(cfg, &mockFetcher{}, newMockDedup(), &mockNotifier{}, testLogger(), Options{})
	if err != nil {
		t.Fatal(err)
	}

	listings := []model.RawListing{
		{Token: "tok", Km: 1000, City: "Haifa", ImageURL: "https://img.com/x.jpg"},
	}
	s.prefillFromDB(context.Background(), listings)

	if listings[0].Km != 1000 {
		t.Error("should not overwrite present fields")
	}
}

func TestBackfillEnrichedListings(t *testing.T) {
	store := pgtest.NewStore(t)
	ctx := context.Background()

	if err := store.UpsertUser(ctx, 100, "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveListing(ctx, storage.ListingRecord{
		Token: "bf-tok", ChatID: 100, SearchName: "s1",
		Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
		Km: 0, City: "", ImageURL: "",
		FirstSeenAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	s, err := NewWithOptions(cfg, &mockFetcher{}, store, &mockNotifier{}, testLogger(), Options{
		ListingStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}

	listings := []model.RawListing{
		{Token: "bf-tok", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000,
			Km: 60000, City: "Eilat", ImageURL: "https://img.com/new.jpg"},
		{Token: "bf-noop", Manufacturer: "Honda", Model: "Civic", Year: 2020, Price: 90000,
			Km: 0},
	}

	s.backfillEnrichedListings(ctx, listings)

	rec, err := store.GetListing(ctx, 100, "bf-tok")
	if err != nil {
		t.Fatal(err)
	}
	if rec == nil {
		t.Fatal("bf-tok should still exist after backfill")
	}
	if rec.Km != 60000 {
		t.Errorf("bf-tok Km = %d, want 60000 after backfill", rec.Km)
	}
	if rec.City != "Eilat" {
		t.Errorf("bf-tok City = %q, want Eilat after backfill", rec.City)
	}
	if rec.ImageURL != "https://img.com/new.jpg" {
		t.Errorf("bf-tok ImageURL = %q, want https://img.com/new.jpg after backfill", rec.ImageURL)
	}
}

func TestBackfillEnrichedListings_Empty(t *testing.T) {
	cfg := testConfig()
	s, err := NewWithOptions(cfg, &mockFetcher{}, newMockDedup(), &mockNotifier{}, testLogger(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	s.backfillEnrichedListings(context.Background(), nil)
}

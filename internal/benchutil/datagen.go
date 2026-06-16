package benchutil

import (
	"fmt"
	"math/rand/v2" //nolint:gosec // deterministic seed for reproducible benchmark data
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type carSpec struct {
	ManufacturerID int
	Manufacturer   string
	ModelID        int
	Model          string
}

var carPool = []carSpec{
	{27, "Mazda", 10332, "Mazda3"},
	{27, "Mazda", 10333, "CX-5"},
	{19, "Toyota", 1, "Corolla"},
	{19, "Toyota", 2, "Yaris"},
	{19, "Toyota", 3, "RAV4"},
	{21, "Honda", 5001, "Civic"},
	{21, "Honda", 5002, "CR-V"},
	{7, "Hyundai", 6001, "i30"},
	{7, "Hyundai", 6002, "Tucson"},
	{22, "Kia", 7001, "Sportage"},
	{22, "Kia", 7002, "Ceed"},
	{10, "Nissan", 8001, "Qashqai"},
	{10, "Nissan", 8002, "Juke"},
	{3, "Volkswagen", 9001, "Golf"},
	{3, "Volkswagen", 9002, "Tiguan"},
	{24, "Skoda", 9501, "Octavia"},
	{24, "Skoda", 9502, "Kodiaq"},
	{14, "Subaru", 10001, "Impreza"},
	{5, "BMW", 11001, "320i"},
	{9, "Mercedes", 12001, "C-Class"},
}

var cities = []string{
	"Tel Aviv", "Jerusalem", "Haifa", "Beer Sheva",
	"Rishon LeZion", "Petah Tikva", "Ashdod", "Netanya",
	"Holon", "Ramat Gan", "Herzliya", "Kfar Saba",
}

var sellerFilters = []string{"any", "any", "any", "any", "any", "any", "any", "private", "private", "commercial"}

// SyntheticUser holds a generated user with their searches.
type SyntheticUser struct {
	ChatID   int64
	Username string
	Searches []storage.Search
}

// GenerateUsers creates n users, each with searchesPerUser searches
// drawn from the car pool with randomized filter ranges.
func GenerateUsers(rng *rand.Rand, n, searchesPerUser int) []SyntheticUser {
	users := make([]SyntheticUser, n)
	for i := range n {
		chatID := int64(100_000 + i)
		username := fmt.Sprintf("bench_user_%04d", i)

		searches := make([]storage.Search, searchesPerUser)
		for j := range searchesPerUser {
			car := carPool[rng.IntN(len(carPool))]
			yearMin := 2016 + rng.IntN(6) // 2016-2021
			yearMax := yearMin + 2 + rng.IntN(4)
			priceMin := 40_000 + rng.IntN(40_000)
			priceMax := priceMin + 30_000 + rng.IntN(50_000)

			var maxKm int
			if rng.IntN(3) > 0 { // 66% have km limit
				maxKm = 80_000 + rng.IntN(120_000)
			}
			var maxHand int
			if rng.IntN(3) > 0 {
				maxHand = 2 + rng.IntN(3)
			}

			searches[j] = storage.Search{
				ChatID:       chatID,
				UserSeq:      j + 1,
				Name:         fmt.Sprintf("%s-%d", car.Model, j+1),
				Manufacturer: car.ManufacturerID,
				Model:        car.ModelID,
				YearMin:      yearMin,
				YearMax:      yearMax,
				PriceMin:     priceMin,
				PriceMax:     priceMax,
				MaxKm:        maxKm,
				MaxHand:      maxHand,
				SellerFilter: sellerFilters[rng.IntN(len(sellerFilters))],
				Active:       true,
			}
		}

		users[i] = SyntheticUser{
			ChatID:   chatID,
			Username: username,
			Searches: searches,
		}
	}
	return users
}

// GenerateListings creates n synthetic listings with realistic field values.
// Listings are spread across the car pool to produce ~30% percolator hit rate.
func GenerateListings(rng *rand.Rand, n int) []model.RawListing {
	listings := make([]model.RawListing, n)
	for i := range n {
		car := carPool[rng.IntN(len(carPool))]
		year := 2016 + rng.IntN(10)
		price := 30_000 + rng.IntN(150_000)
		km := rng.IntN(250_000)
		hand := 1 + rng.IntN(4)
		isCommercial := rng.IntN(7) == 0 // ~15% commercial
		commercial := &isCommercial

		listings[i] = model.RawListing{
			Token:          fmt.Sprintf("bench-tok-%06d", i),
			Manufacturer:   car.Manufacturer,
			ManufacturerID: car.ManufacturerID,
			Model:          car.Model,
			ModelID:        car.ModelID,
			Year:           year,
			Price:          price,
			Km:             km,
			Hand:           hand,
			EngineVolume:   1400 + float64(rng.IntN(1200)),
			GearBox:        []string{"אוטומט", "ידני", "רובוטית"}[rng.IntN(3)],
			City:           cities[rng.IntN(len(cities))],
			ImageURL:       fmt.Sprintf("https://img.yad2.co.il/bench/%d.jpg", i),
			PageLink:       fmt.Sprintf("https://www.yad2.co.il/item/bench-%06d", i),
			Commercial:     commercial,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}
	return listings
}

// AllSearches flattens users into a single search slice.
func AllSearches(users []SyntheticUser) []storage.Search {
	var all []storage.Search
	for _, u := range users {
		all = append(all, u.Searches...)
	}
	return all
}

// ToListingRecords converts raw listings into storage records for DB seeding.
func ToListingRecords(listings []model.RawListing, chatID, searchID int64, searchName string) []storage.ListingRecord {
	records := make([]storage.ListingRecord, len(listings))
	for i, l := range listings {
		fitness := 5.0 + float64(i%10)*0.5
		records[i] = storage.ListingRecord{
			Token:        l.Token,
			ChatID:       chatID,
			SearchID:     searchID,
			SearchName:   searchName,
			Manufacturer: l.Manufacturer,
			Model:        l.Model,
			Year:         l.Year,
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			City:         l.City,
			PageLink:     l.PageLink,
			ImageURL:     l.ImageURL,
			EngineVolume: l.EngineVolume,
			GearBox:      l.GearBox,
			IsCommercial: l.Commercial,
			FitnessScore: &fitness,
			FirstSeenAt:  time.Now(),
		}
	}
	return records
}

// GenerateMedianEntries creates synthetic median entries for market cache benchmarks.
func GenerateMedianEntries(rng *rand.Rand, n int) []MedianEntry {
	entries := make([]MedianEntry, n)
	for i := range n {
		car := carPool[rng.IntN(len(carPool))]
		entries[i] = MedianEntry{
			Manufacturer: car.Manufacturer,
			Model:        car.Model,
			Year:         2016 + rng.IntN(10),
			MedianPrice:  50_000 + rng.IntN(100_000),
			MedianKm:     30_000 + rng.IntN(150_000),
			CohortSize:   10 + rng.IntN(90),
		}
	}
	return entries
}

// MedianEntry mirrors scoring.MedianEntry but avoids a circular import.
type MedianEntry struct {
	Manufacturer string
	Model        string
	Year         int
	MedianPrice  int
	MedianKm     int
	CohortSize   int
}

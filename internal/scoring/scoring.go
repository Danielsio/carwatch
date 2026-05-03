package scoring

import (
	"math"
	"sort"
	"strings"
)

const MinCohortSize = 10

type ListingData struct {
	Manufacturer string
	Model        string
	Year         int
	Price        int
}

type entry struct {
	Year  int
	Price int
}

type MarketCache struct {
	data map[string][]entry
}

func NewMarketCache(listings []ListingData) *MarketCache {
	m := make(map[string][]entry)
	for _, l := range listings {
		key := cacheKey(l.Manufacturer, l.Model)
		m[key] = append(m[key], entry{Year: l.Year, Price: l.Price})
	}
	return &MarketCache{data: m}
}

func (mc *MarketCache) Lookup(manufacturer, model string, year int) (median int, cohortSize int, ok bool) {
	entries := mc.data[cacheKey(manufacturer, model)]
	var prices []int
	for _, e := range entries {
		if abs(e.Year-year) <= 1 {
			prices = append(prices, e.Price)
		}
	}
	if len(prices) < MinCohortSize {
		return 0, len(prices), false
	}
	sort.Ints(prices)
	n := len(prices)
	if n%2 == 0 {
		median = (prices[n/2-1] + prices[n/2]) / 2
	} else {
		median = prices[n/2]
	}
	return median, n, true
}

func Score(listingPrice, medianPrice int) int {
	if medianPrice <= 0 || listingPrice <= 0 {
		return 0
	}
	raw := 100.0 * (1.0 - float64(listingPrice)/float64(medianPrice))
	if raw < 0 {
		return 0
	}
	if raw > 100 {
		return 100
	}
	return int(math.Round(raw))
}

func cacheKey(manufacturer, model string) string {
	return strings.ToLower(manufacturer) + "|" + strings.ToLower(model)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

const (
	weightPrice  = 0.35
	weightKm     = 0.25
	weightHand   = 0.20
	weightYear   = 0.15
	weightEngine = 0.05

	defaultMaxKm = 200000

	curveKm    = 1.5
	curvePrice = 1.5
	curveHand  = 0.6
)

type FitnessParams struct {
	Price        int
	Km           int
	Hand         int
	Year         int
	EngineVolume float64

	PriceMax    int
	MaxKm       int
	MaxHand     int
	YearMin     int
	YearMax     int
	EngineMinCC int
}

type DimScore struct {
	Name   string
	Score  float64
	Weight float64
}

type FitnessResult struct {
	Total float64
	Dims  []DimScore
}

func FitnessScore(p FitnessParams) float64 {
	return FitnessScoreDetailed(p).Total
}

func FitnessScoreDetailed(p FitnessParams) FitnessResult {
	dims := make([]DimScore, 0, 5)

	if p.PriceMax > 0 && p.Price > 0 {
		dims = append(dims, DimScore{"price", priceScore(p.Price, p.PriceMax), weightPrice})
	}

	dims = append(dims, DimScore{"km", kmScore(p.Km, p.MaxKm), weightKm})
	dims = append(dims, DimScore{"hand", handScore(p.Hand, p.MaxHand), weightHand})
	dims = append(dims, DimScore{"year", yearScore(p.Year, p.YearMin, p.YearMax), weightYear})
	dims = append(dims, DimScore{"engine", engineScore(p.EngineVolume, p.EngineMinCC), weightEngine})

	var totalWeight float64
	for _, d := range dims {
		totalWeight += d.Weight
	}
	if totalWeight <= 0 {
		return FitnessResult{Total: 5.0, Dims: dims}
	}

	var weighted float64
	for _, d := range dims {
		weighted += (d.Weight / totalWeight) * d.Score
	}

	raw := weighted * 10.0
	total := math.Round(raw*10) / 10

	return FitnessResult{Total: total, Dims: dims}
}

func priceScore(price, priceMax int) float64 {
	if priceMax <= 0 {
		return 0.5
	}
	ratio := float64(price) / float64(priceMax)
	s := 1.0 - math.Pow(clamp01(ratio), curvePrice)
	return clamp01(s)
}

func kmScore(km, maxKm int) float64 {
	if km <= 0 {
		return 0.5
	}
	ref := maxKm
	if ref <= 0 {
		ref = defaultMaxKm
	}
	ratio := float64(km) / float64(ref)
	s := 1.0 - math.Pow(clamp01(ratio), curveKm)
	return clamp01(s)
}

func handScore(hand, maxHand int) float64 {
	if hand <= 0 {
		return 0.5
	}
	if maxHand > 0 {
		ratio := float64(hand-1) / float64(maxHand)
		s := 1.0 - math.Pow(clamp01(ratio), curveHand)
		return clamp01(s)
	}
	switch hand {
	case 1:
		return 1.0
	case 2:
		return 0.7
	case 3:
		return 0.4
	default:
		return 0.1
	}
}

func yearScore(year, yearMin, yearMax int) float64 {
	if yearMin <= 0 || yearMax <= 0 || yearMax <= yearMin {
		return 1.0
	}
	s := float64(year-yearMin) / float64(yearMax-yearMin)
	return clamp01(s)
}

func engineScore(engineVolume float64, engineMinCC int) float64 {
	if engineMinCC <= 0 {
		return 1.0
	}
	if engineVolume <= 0 {
		return 0.5
	}
	s := (engineVolume - float64(engineMinCC)) / float64(engineMinCC)
	return clamp01(math.Min(s, 1.0))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

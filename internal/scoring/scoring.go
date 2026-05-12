package scoring

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	MinCohortSize = 10
	minPriceFloor = 5000 // ignore placeholder prices below this (NIS)
)

type ListingData struct {
	Manufacturer string
	Model        string
	Year         int
	Price        int
	Km           int
}

type entry struct {
	Year  int
	Price int
	Km    int
}

type MarketCache struct {
	data map[string][]entry
	sf   singleflight.Group
}

func NewMarketCache(listings []ListingData) *MarketCache {
	m := make(map[string][]entry)
	for _, l := range listings {
		key := cacheKey(l.Manufacturer, l.Model)
		m[key] = append(m[key], entry{Year: l.Year, Price: l.Price, Km: l.Km})
	}
	return &MarketCache{data: m}
}

type lookupResult struct {
	median     int
	medianKm   int
	cohortSize int
	ok         bool
}

func cohortLookupKey(manufacturer, model string, year int) string {
	return fmt.Sprintf("%s|%d", cacheKey(manufacturer, model), year)
}

// Lookup returns the median price, median km, cohort size, and whether enough data exists.
func (mc *MarketCache) Lookup(manufacturer, model string, year int) (median int, medianKm int, cohortSize int, ok bool) {
	key := cohortLookupKey(manufacturer, model, year)
	v, _, _ := mc.sf.Do(key, func() (interface{}, error) {
		return mc.lookupUnsynchronized(manufacturer, model, year), nil
	})
	res := v.(lookupResult)
	return res.median, res.medianKm, res.cohortSize, res.ok
}

func (mc *MarketCache) lookupUnsynchronized(manufacturer, model string, year int) lookupResult {
	entries := mc.data[cacheKey(manufacturer, model)]
	var prices, kms []int
	for _, e := range entries {
		if abs(e.Year-year) <= 1 && e.Price >= minPriceFloor {
			prices = append(prices, e.Price)
			if e.Km > 0 {
				kms = append(kms, e.Km)
			}
		}
	}
	if len(prices) < MinCohortSize {
		return lookupResult{cohortSize: len(prices)}
	}
	medianKm := 0
	if len(kms) >= 3 {
		medianKm = medianInt(kms)
	}
	return lookupResult{
		median:     medianInt(prices),
		medianKm:   medianKm,
		cohortSize: len(prices),
		ok:         true,
	}
}

func medianInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sort.Ints(vals)
	n := len(vals)
	if n%2 == 0 {
		return (vals[n/2-1] + vals[n/2]) / 2
	}
	return vals[n/2]
}

// Score computes a deal score (0-100) based on how far the listing price is
// below the median. Higher = better deal.
func Score(listingPrice, medianPrice int) int {
	return scoreRaw(listingPrice, medianPrice)
}

// ScoreWithKm computes a km-adjusted deal score. When both the listing and
// cohort have km data, a low-km car priced at median gets a bonus and a
// high-km car priced at median gets a penalty. When the cohort has km data
// but the listing does not, the price-only score is reduced slightly (floored
// at 0). Otherwise falls back to price-only scoring when cohort km is unavailable.
func ScoreWithKm(listingPrice, listingKm, medianPrice, medianKm int) int {
	base := scoreRaw(listingPrice, medianPrice)
	if medianKm <= 0 || listingKm <= 0 {
		if medianKm > 0 && listingKm <= 0 {
			// Unknown mileage gets a small penalty when the market cohort has km data.
			penalized := base - 5
			if penalized < 0 {
				return 0
			}
			return penalized
		}
		return base
	}
	// Km adjustment: if listing has fewer km than median, it's a better deal.
	// Scale: 50% fewer km -> +10 bonus, 50% more km -> -10 penalty.
	kmRatio := float64(listingKm) / float64(medianKm)
	kmAdj := (1.0 - kmRatio) * 20.0 // +/-20 points for double/half km
	adjusted := float64(base) + kmAdj
	if adjusted < 0 {
		return 0
	}
	if adjusted > 100 {
		return 100
	}
	return int(math.Round(adjusted))
}

func scoreRaw(listingPrice, medianPrice int) int {
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
	weightPrice  = 0.25
	weightKm     = 0.30
	weightHand   = 0.20
	weightYear   = 0.20
	weightEngine = 0.05

	avgKmPerYear       = 15000 // Israeli average annual mileage
	ageAdjustKmBlend   = 0.6  // weight of age-adjusted vs cap-based km score
	kmAgeExponent      = 1.2  // age-adjusted km scoring curve
	kmCapExponent      = 1.5  // cap-based km scoring curve
	handAgeBonusMax    = 0.15 // max hand score bonus for older cars
	handAgeBonusYears  = 15.0 // car age at which full bonus is reached
	yearScoreFloor     = 0.3  // minimum year score within range
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

	// MedianPrice is the market median price for this car's manufacturer+model+year
	// cohort. When > 0 the price dimension scores against market value instead of
	// the user's budget cap.
	MedianPrice int
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

// Dimension defines one fitness scoring strategy (name, weight, score function).
type Dimension struct {
	Name   string
	Weight float64
	Score  func(p FitnessParams) float64
}

// DefaultDimensions returns the standard fitness dimensions (price, km, hand, year, engine).
// A score function may return NaN to omit that dimension (used for price when unset).
func DefaultDimensions() []Dimension {
	return []Dimension{
		{
			Name:   "price",
			Weight: weightPrice,
			Score: func(p FitnessParams) float64 {
				if p.Price <= 0 {
					return math.NaN()
				}
				if p.MedianPrice > 0 {
					return marketPriceScore(p.Price, p.MedianPrice)
				}
				if p.PriceMax <= 0 {
					return math.NaN()
				}
				return budgetPriceScore(p.Price, p.PriceMax)
			},
		},
		{Name: "km", Weight: weightKm, Score: func(p FitnessParams) float64 { return kmScore(p.Km, p.MaxKm, p.Year) }},
		{Name: "hand", Weight: weightHand, Score: func(p FitnessParams) float64 { return handScore(p.Hand, p.MaxHand, p.Year) }},
		{Name: "year", Weight: weightYear, Score: func(p FitnessParams) float64 { return yearScore(p.Year, p.YearMin, p.YearMax) }},
		{Name: "engine", Weight: weightEngine, Score: func(p FitnessParams) float64 {
			return engineScore(p.EngineVolume, p.EngineMinCC)
		}},
	}
}

var defaultFitnessDimensions []Dimension

func init() {
	defaultFitnessDimensions = DefaultDimensions()
}

func FitnessScore(p FitnessParams) float64 {
	return FitnessScoreDetailed(p).Total
}

func FitnessScoreDetailed(p FitnessParams) FitnessResult {
	dims := make([]DimScore, 0, 5)

	for _, dim := range defaultFitnessDimensions {
		s := dim.Score(p)
		if math.IsNaN(s) {
			continue
		}
		dims = append(dims, DimScore{Name: dim.Name, Score: s, Weight: dim.Weight})
	}

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

// marketPriceScore scores the listing price against the market median.
// Maps [0.7×median, 1.3×median] → [1.0, 0.0] via a sqrt curve.
// At median the score is ~0.71 (fair asking price); 30%+ above is 0.
func marketPriceScore(price, medianPrice int) float64 {
	ratio := float64(price) / float64(medianPrice)
	normalized := (ratio - 0.7) / 0.6 // 0.7→0, 1.0→0.5, 1.3→1.0
	normalized = clamp01(normalized)
	return math.Sqrt(1.0 - normalized)
}

// budgetPriceScore: cheaper within budget = better value (legacy fallback).
// sqrt curve so savings have diminishing returns — 50% of budget is great,
// 80% is decent, at-cap is low but not zero.
func budgetPriceScore(price, priceMax int) float64 {
	if priceMax <= 0 {
		return 0.5
	}
	ratio := float64(price) / float64(priceMax)
	if ratio >= 1.0 {
		return 0.0
	}
	return math.Sqrt(1.0 - ratio)
}

// kmScore: blends age-adjusted km expectations with an absolute cap score.
// A 12-year-old car with 82k km (~7k/yr vs 15k avg) scores well;
// the same 82k on a 2-year-old car scores poorly.
func kmScore(km, maxKm, carYear int) float64 {
	if km <= 0 {
		return math.NaN()
	}

	var ageScore float64
	hasAge := carYear > 0
	if hasAge {
		carAge := currentYear() - carYear
		if carAge < 1 {
			carAge = 1
		}
		expectedKm := float64(carAge * avgKmPerYear)
		kmRatio := float64(km) / expectedKm
		ageScore = clamp01(1.0 - math.Pow(clamp01(kmRatio), kmAgeExponent))
	}

	if maxKm > 0 {
		capRatio := float64(km) / float64(maxKm)
		capScore := clamp01(1.0 - math.Pow(clamp01(capRatio), kmCapExponent))
		if !hasAge {
			return capScore
		}
		return ageAdjustKmBlend*ageScore + (1-ageAdjustKmBlend)*capScore
	}
	if !hasAge {
		return math.NaN()
	}
	return ageScore
}

// handScore: base ladder plus a bonus for older cars where more owners are expected.
func handScore(hand, maxHand, carYear int) float64 {
	if hand <= 0 {
		return 0.5
	}
	var base float64
	if maxHand > 0 {
		ratio := float64(hand-1) / float64(maxHand)
		base = clamp01(1.0 - math.Pow(clamp01(ratio), 0.6))
	} else {
		switch hand {
		case 1:
			base = 1.0
		case 2:
			base = 0.7
		case 3:
			base = 0.4
		default:
			base = 0.1
		}
	}
	// Older cars naturally have more owners; don't penalize as harshly.
	if hand > 1 && carYear > 0 {
		age := float64(currentYear() - carYear)
		if age < 1 {
			age = 1
		}
		bonus := clamp01(age/handAgeBonusYears) * handAgeBonusMax
		base = clamp01(base + bonus)
	}
	return base
}

// yearScore: position within search range with a floor so older-in-range
// cars aren't crushed — price and km already capture age effects.
func yearScore(year, yearMin, yearMax int) float64 {
	if yearMin <= 0 || yearMax <= 0 || yearMax <= yearMin {
		return 1.0
	}
	pos := clamp01(float64(year-yearMin) / float64(yearMax-yearMin))
	return yearScoreFloor + (1.0-yearScoreFloor)*math.Sqrt(pos)
}

// currentYear returns the calendar year. Extracted for testability.
var currentYear = func() int {
	return time.Now().Year()
}

func engineScore(engineVolume float64, engineMinCC int) float64 {
	if engineMinCC <= 0 {
		return 1.0
	}
	if engineVolume <= 0 {
		return 0.5
	}
	minCC := float64(engineMinCC)
	if engineVolume < minCC {
		return 0.0
	}
	// Meeting the minimum scores 0.7; 50% above minimum reaches 1.0.
	bonus := (engineVolume - minCC) / (minCC * 0.5)
	return clamp01(0.7 + 0.3*bonus)
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

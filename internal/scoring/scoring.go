package scoring

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	MinCohortSize = 10
	minPriceFloor = 5000 // ignore placeholder prices below this (NIS)
)

// ListingData holds a raw listing for building a MarketCache from unprocessed
// listings (used by instant search). For the scheduler path prefer
// NewMarketCacheFromMedians which accepts pre-aggregated data.
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

// MedianEntry holds pre-computed median data for a single
// (manufacturer, model, year) cohort from the materialized view.
type MedianEntry struct {
	Manufacturer string
	Model        string
	Year         int
	MedianPrice  int
	MedianKm     int
	CohortSize   int
}

// MarketCache provides market median lookups.
//
// Two construction paths exist:
//   - NewMarketCache: builds from raw listings; Lookup performs year-band
//     matching (+/- 1 year) and computes medians on demand.
//   - NewMarketCacheFromMedians: builds from pre-aggregated medians
//     (materialized view); Lookup does a direct map hit on exact year.
type MarketCache struct {
	// raw path: populated by NewMarketCache
	raw map[string][]entry

	// precomputed path: populated by NewMarketCacheFromMedians
	precomputed map[string]lookupResult
}

// NewMarketCache builds a cache from raw listings. Lookup computes medians
// lazily using a year +/- 1 band (used by instant search).
func NewMarketCache(listings []ListingData) *MarketCache {
	m := make(map[string][]entry, len(listings))
	for _, l := range listings {
		key := cacheKey(l.Manufacturer, l.Model)
		m[key] = append(m[key], entry{Year: l.Year, Price: l.Price, Km: l.Km})
	}
	return &MarketCache{raw: m}
}

// NewMarketCacheFromMedians builds a cache from pre-computed medians (e.g.
// loaded from the market_medians materialized view). Lookup is a direct map
// hit on the exact (manufacturer, model, year) key.
func NewMarketCacheFromMedians(entries []MedianEntry) *MarketCache {
	m := make(map[string]lookupResult, len(entries))
	for _, e := range entries {
		key := medianKey(e.Manufacturer, e.Model, e.Year)
		m[key] = lookupResult{
			median:     e.MedianPrice,
			medianKm:   e.MedianKm,
			cohortSize: e.CohortSize,
			ok:         e.CohortSize >= MinCohortSize,
		}
	}
	return &MarketCache{precomputed: m}
}

type lookupResult struct {
	median     int
	medianKm   int
	cohortSize int
	ok         bool
}

func medianKey(manufacturer, model string, year int) string {
	return strings.ToLower(manufacturer) + "|" + strings.ToLower(model) + "|" + strconv.Itoa(year)
}

// Lookup returns the median price, median km, cohort size, and whether enough data exists.
// For precomputed data, it checks year ±1 band and aggregates cohorts (matching the raw path behavior).
func (mc *MarketCache) Lookup(manufacturer, model string, year int) (median int, medianKm int, cohortSize int, ok bool) {
	if mc.precomputed != nil {
		return mc.lookupPrecomputed(manufacturer, model, year)
	}
	res := mc.computeFromRaw(manufacturer, model, year)
	return res.median, res.medianKm, res.cohortSize, res.ok
}

func (mc *MarketCache) lookupPrecomputed(manufacturer, model string, year int) (int, int, int, bool) {
	var totalPrice, totalKm, totalCohort int
	var count int
	for dy := -1; dy <= 1; dy++ {
		res, found := mc.precomputed[medianKey(manufacturer, model, year+dy)]
		if found && res.ok {
			totalPrice += res.median * res.cohortSize
			totalKm += res.medianKm * res.cohortSize
			totalCohort += res.cohortSize
			count++
		}
	}
	if totalCohort < MinCohortSize {
		return 0, 0, 0, false
	}
	return totalPrice / totalCohort, totalKm / totalCohort, totalCohort, true
}

func (mc *MarketCache) computeFromRaw(manufacturer, model string, year int) lookupResult {
	entries := mc.raw[cacheKey(manufacturer, model)]
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

// BES v2 (Buyer's Excitement Score) — additive baseline model.
// final = clamp(5.0 + conditionDelta + valueDelta + engineDelta, 0, 10)
const (
	baselineScore = 5.0

	// Condition: km sub-delta
	expectedKmPerYear = 15000
	kmLowThreshold    = 0.85
	kmLowBonusScale   = 4.2
	kmDeadZonePenalty = 0.3
	kmExcessSteepness = 4.7
	kmExcessCap       = 1.5
	kmExcessExponent  = 0.7

	// Condition: hand sub-delta
	hand1BonusBase             = 0.5
	hand1AgeBonusRate          = 0.10
	hand1AgeBonusCap           = 0.8
	handExpectedRate           = 5.0
	handExcessPenalty          = 1.1
	handBelowRate              = 0.3
	handBelowCap               = 0.3
	commercialHand1BonusFactor = 0.4

	// Ownership origin discounts (value dimension)
	leaseOriginPriceDisc  = 0.04
	rentalOriginPriceDisc = 0.06

	// Condition delta bounds
	condDeltaMin = -5.0
	condDeltaMax = 4.5

	// Value delta
	valDeltaMin           = -3.5
	valDeltaMax           = 2.5
	valCondGateThreshold  = 0.55
	valNeutralLo          = 0.92
	valNeutralHi          = 1.05
	valNeutralMaxPenalty  = 0.3
	valOverpaySteepness   = 3.2
	valOverpaySpan        = 0.30
	kmPriceAdjFactor      = 0.15
	handFirstPricePremium = 0.05
	handExcessPriceDisc   = 0.07

	// Budget fallback
	budgetHighHeadroom = 0.30
	budgetHighBonus    = 2.0
	budgetLowHeadroom  = 0.10
	budgetLowPenalty   = 0.5
	budgetOverPenalty  = 1.0

	// Pricelist (BasePrice) fallback — wider tolerance, dampened bounds
	basePriceNeutralLo      = 0.88
	basePriceNeutralHi      = 1.10
	basePriceMaxBonus       = 1.8
	basePriceMaxPenalty     = -2.8
	basePriceNeutralPenalty = 0.2
	basePriceOverpaySpan    = 0.30
	basePriceKmAdjFactor    = 0.12

	// Engine delta
	engineDeltaMax = 0.1

	// UI breakdown weights (for display proportionality only)
	condWeight   = 0.60
	valWeight    = 0.35
	engineWeight = 0.05
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

	MedianPrice int
	MedianKm    int
	BasePrice   int

	IsCommercial      bool
	OriginalOwnership string
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

// DefaultDimensions returns the BES v2 dimensions (condition, value, engine).
func DefaultDimensions() []Dimension {
	return []Dimension{
		{Name: "condition", Weight: condWeight},
		{Name: "value", Weight: valWeight},
		{Name: "engine", Weight: engineWeight},
	}
}

func FitnessScore(p FitnessParams) float64 {
	return FitnessScoreDetailed(p).Total
}

func FitnessScoreDetailed(p FitnessParams) FitnessResult {
	carAge := 1
	if p.Year > 0 {
		carAge = currentYear() - p.Year
		if carAge < 1 {
			carAge = 1
		}
	}

	kmDelta, kmScore01 := computeKmDelta(p.Km, carAge)
	handDelta, handScore01 := computeHandDelta(p.Hand, carAge, p.IsCommercial)
	condDelta := clampRange(kmDelta+handDelta, condDeltaMin, condDeltaMax)
	condScore01 := 0.70*kmScore01 + 0.30*handScore01

	valDelta, valScore01 := computeValueDelta(p, carAge, condScore01)
	engDelta, engScore01 := computeEngineDelta(p.EngineVolume, p.EngineMinCC)

	if p.EngineMinCC > 0 && p.EngineVolume > 0 && p.EngineVolume < float64(p.EngineMinCC) {
		return FitnessResult{
			Total: 0,
			Dims: []DimScore{
				{Name: "condition", Score: condScore01, Weight: condWeight},
				{Name: "value", Score: valScore01, Weight: valWeight},
				{Name: "engine", Score: 0, Weight: engineWeight},
			},
		}
	}

	raw := baselineScore + condDelta + valDelta + engDelta
	total := math.Round(clampRange(raw, 0, 10)*10) / 10

	return FitnessResult{
		Total: total,
		Dims: []DimScore{
			{Name: "condition", Score: condScore01, Weight: condWeight},
			{Name: "value", Score: valScore01, Weight: valWeight},
			{Name: "engine", Score: engScore01, Weight: engineWeight},
		},
	}
}

func computeKmDelta(km int, carAge int) (delta float64, score01 float64) {
	if km <= 0 {
		return 0, 0.5
	}

	expectedKm := float64(carAge) * expectedKmPerYear
	kmRatio := float64(km) / expectedKm

	switch {
	case kmRatio <= kmLowThreshold:
		delta = kmLowBonusScale * math.Sqrt((kmLowThreshold-kmRatio)/kmLowThreshold)
	case kmRatio <= 1.0:
		delta = -kmDeadZonePenalty * (kmRatio - kmLowThreshold) / (1.0 - kmLowThreshold)
	default:
		excess := math.Min(kmRatio-1.0, kmExcessCap)
		delta = -kmDeadZonePenalty - kmExcessSteepness*math.Pow(excess, kmExcessExponent)
	}

	if kmRatio <= 1.0 {
		score01 = 1.0 / (1.0 + math.Exp(4.0*(kmRatio-0.7)))
	} else {
		score01 = 1.0 / (1.0 + math.Exp(5.0*(kmRatio-1.0)))
	}

	return delta, score01
}

func computeHandDelta(hand int, carAge int, isCommercial bool) (delta float64, score01 float64) {
	if hand <= 0 {
		return 0, 0.5
	}

	if hand == 1 {
		ageBonus := math.Min(hand1AgeBonusCap, float64(carAge)*hand1AgeBonusRate)
		delta = hand1BonusBase + ageBonus
		if isCommercial {
			delta *= commercialHand1BonusFactor
			return delta, 0.7
		}
		return delta, 1.0
	}

	expectedHands := 1.0 + float64(carAge)/handExpectedRate
	handExcess := float64(hand) - expectedHands

	if handExcess <= 0 {
		delta = clampRange(-handExcess*handBelowRate, 0, handBelowCap)
		score01 = clamp01(0.7 + (-handExcess)*0.15)
	} else {
		delta = -handExcessPenalty * handExcess
		score01 = clamp01(0.5 - handExcess*0.25)
	}

	return delta, score01
}

func computeValueDelta(p FitnessParams, carAge int, condScore01 float64) (delta float64, score01 float64) {
	if p.Price <= 0 {
		return 0, 0.5
	}

	if p.MedianPrice > 0 {
		adjExpected := float64(p.MedianPrice)

		if p.MedianKm > 0 && p.Km > 0 {
			kmDeltaPct := float64(p.MedianKm-p.Km) / float64(p.MedianKm)
			adjExpected *= clampRange(1.0+kmDeltaPct*kmPriceAdjFactor, 0.5, 1.5)
		}

		expectedHands := 1.0 + float64(carAge)/handExpectedRate
		if p.Hand == 1 && carAge >= 3 {
			adjExpected *= (1.0 + handFirstPricePremium)
		} else if p.Hand > 0 && float64(p.Hand) > expectedHands {
			excess := float64(p.Hand) - expectedHands
			adjExpected *= math.Max(0.85, 1.0-excess*handExcessPriceDisc)
		}

		switch p.OriginalOwnership {
		case "lease":
			adjExpected *= (1.0 - leaseOriginPriceDisc)
		case "rental":
			adjExpected *= (1.0 - rentalOriginPriceDisc)
		}

		ratio := float64(p.Price) / adjExpected

		switch {
		case ratio < 0.80:
			delta = valDeltaMax
		case ratio < valNeutralLo:
			delta = valDeltaMax * (valNeutralLo - ratio) / (valNeutralLo - 0.80)
		case ratio <= valNeutralHi:
			delta = -valNeutralMaxPenalty * (ratio - valNeutralLo) / (valNeutralHi - valNeutralLo)
		default:
			overpay := (ratio - valNeutralHi) / valOverpaySpan
			delta = clampRange(-valNeutralMaxPenalty-valOverpaySteepness*math.Pow(math.Min(overpay, 1.0), 0.7), valDeltaMin, -valNeutralMaxPenalty)
		}

		if delta > 0 && condScore01 < valCondGateThreshold {
			delta *= condScore01 / valCondGateThreshold
		}
	} else if p.BasePrice > 0 {
		adjExpected := float64(p.BasePrice) * basePriceDiscount(carAge)

		if p.Km > 0 && carAge > 0 {
			expectedKm := float64(carAge) * expectedKmPerYear
			kmDeltaPct := (expectedKm - float64(p.Km)) / expectedKm
			adjExpected *= clampRange(1.0+kmDeltaPct*basePriceKmAdjFactor, 0.70, 1.30)
		}

		expectedHands := 1.0 + float64(carAge)/handExpectedRate
		if p.Hand == 1 && carAge >= 3 {
			adjExpected *= (1.0 + handFirstPricePremium)
		} else if p.Hand > 0 && float64(p.Hand) > expectedHands {
			excess := float64(p.Hand) - expectedHands
			adjExpected *= math.Max(0.85, 1.0-excess*handExcessPriceDisc)
		}

		switch p.OriginalOwnership {
		case "lease":
			adjExpected *= (1.0 - leaseOriginPriceDisc)
		case "rental":
			adjExpected *= (1.0 - rentalOriginPriceDisc)
		}

		ratio := float64(p.Price) / adjExpected

		switch {
		case ratio < 0.75:
			delta = basePriceMaxBonus
		case ratio < basePriceNeutralLo:
			delta = basePriceMaxBonus * (basePriceNeutralLo - ratio) / (basePriceNeutralLo - 0.75)
		case ratio <= basePriceNeutralHi:
			delta = -basePriceNeutralPenalty * (ratio - basePriceNeutralLo) / (basePriceNeutralHi - basePriceNeutralLo)
		default:
			overpay := (ratio - basePriceNeutralHi) / basePriceOverpaySpan
			delta = clampRange(-basePriceNeutralPenalty-(-basePriceMaxPenalty-basePriceNeutralPenalty)*math.Pow(math.Min(overpay, 1.0), 0.7), basePriceMaxPenalty, -basePriceNeutralPenalty)
		}

		if delta > 0 && condScore01 < valCondGateThreshold {
			delta *= condScore01 / valCondGateThreshold
		}
	} else if p.PriceMax > 0 {
		headroom := 1.0 - float64(p.Price)/float64(p.PriceMax)
		switch {
		case headroom >= budgetHighHeadroom:
			delta = budgetHighBonus
		case headroom >= budgetLowHeadroom:
			delta = budgetHighBonus * (headroom - budgetLowHeadroom) / (budgetHighHeadroom - budgetLowHeadroom)
		case headroom >= 0:
			delta = -budgetLowPenalty * (budgetLowHeadroom - headroom) / budgetLowHeadroom
		default:
			delta = -budgetOverPenalty
		}
	} else {
		return 0, 0.5
	}

	delta = clampRange(delta, valDeltaMin, valDeltaMax)
	score01 = clamp01(0.5 + delta/5.0)
	return delta, score01
}

func basePriceDiscount(carAge int) float64 {
	switch {
	case carAge <= 1:
		return 0.97
	case carAge <= 3:
		return 0.94
	case carAge <= 6:
		return 0.90
	default:
		return 0.85
	}
}

func computeEngineDelta(engineVolume float64, engineMinCC int) (delta float64, score01 float64) {
	if engineMinCC <= 0 {
		return 0, 1.0
	}
	if engineVolume <= 0 {
		return 0, 0.5
	}
	minCC := float64(engineMinCC)
	if engineVolume < minCC {
		return 0, 0
	}
	bonus := (engineVolume - minCC) / (minCC * 0.5)
	score01 = clamp01(0.7 + 0.3*bonus)
	delta = engineDeltaMax * clamp01(bonus)
	return delta, score01
}

// currentYear returns the calendar year. Extracted for testability.
var currentYear = func() int {
	return time.Now().Year()
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

func clampRange(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

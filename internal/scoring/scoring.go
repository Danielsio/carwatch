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

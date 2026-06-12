// Package percolator implements reverse-search matching: given a raw listing,
// it finds all active searches (rules) that the listing satisfies.  This
// replaces the old group-by-(manufacturer,model) approach with a single
// global feed scan.
package percolator

import (
	"strings"
	"sync"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

// MatchResult describes a single search that matched a listing.
type MatchResult struct {
	SearchID   int64
	ChatID     int64
	SearchName string
	Search     storage.Search
}

// Percolator holds a snapshot of active searches and matches incoming
// listings against all of them.
type Percolator struct {
	mu    sync.RWMutex
	rules []storage.Search
}

// New creates an empty Percolator.
func New() *Percolator {
	return &Percolator{}
}

// Load replaces the current rule set with the given searches.
// It is safe to call concurrently with Match.
func (p *Percolator) Load(searches []storage.Search) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = make([]storage.Search, len(searches))
	copy(p.rules, searches)
}

// Match returns every search that the listing satisfies.
func (p *Percolator) Match(listing model.RawListing) []MatchResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var matches []MatchResult
	for _, s := range p.rules {
		if matchesSearch(listing, s) {
			matches = append(matches, MatchResult{
				SearchID:   s.ID,
				ChatID:     s.ChatID,
				SearchName: s.Name,
				Search:     s,
			})
		}
	}
	return matches
}

// RejectReason identifies why a listing did not match a search.
type RejectReason string

const (
	RejectWrongModel  RejectReason = "wrong_model"
	RejectYearOut     RejectReason = "year_out"
	RejectPriceOut    RejectReason = "price_out"
	RejectKmOver      RejectReason = "km_over"
	RejectHandOver    RejectReason = "hand_over"
	RejectEngineCC    RejectReason = "engine_cc"
	RejectSeller      RejectReason = "seller"
	RejectOtherFilter RejectReason = "other_filter"
)

// CountRejections returns per-search rejection counts for all listings.
// Each non-matching listing is counted under its primary rejection reason
// (the first filter it fails). Matched listings are not counted.
func (p *Percolator) CountRejections(listings []model.RawListing) map[int64]map[RejectReason]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[int64]map[RejectReason]int, len(p.rules))
	for _, s := range p.rules {
		counts := make(map[RejectReason]int)
		for i := range listings {
			if reason := classifyRejection(listings[i], s); reason != "" {
				counts[reason]++
			}
		}
		result[s.ID] = counts
	}
	return result
}

// classifyRejection returns the primary reason a listing does not match a
// search, or "" if it matches. The check order mirrors matchesSearch.
func classifyRejection(l model.RawListing, s storage.Search) RejectReason {
	if s.Manufacturer > 0 && l.ManufacturerID != s.Manufacturer {
		return RejectWrongModel
	}
	if s.Model > 0 && l.ModelID != s.Model {
		return RejectWrongModel
	}
	if s.YearMin > 0 && l.Year < s.YearMin {
		return RejectYearOut
	}
	if s.YearMax > 0 && l.Year > s.YearMax {
		return RejectYearOut
	}
	if s.PriceMin > 0 && l.Price > 0 && l.Price < s.PriceMin {
		return RejectPriceOut
	}
	if s.PriceMax > 0 && l.Price > s.PriceMax {
		return RejectPriceOut
	}
	if s.MaxKm > 0 && l.Km > 0 && l.Km > s.MaxKm {
		return RejectKmOver
	}
	if s.MaxHand > 0 && l.Hand > s.MaxHand {
		return RejectHandOver
	}
	if s.EngineMinCC > 0 && int(l.EngineVolume) < s.EngineMinCC {
		return RejectEngineCC
	}
	if s.GearBox != "" && l.GearBox != "" {
		if !strings.EqualFold(s.GearBox, l.GearBox) {
			return RejectOtherFilter
		}
	}
	sf := strings.ToLower(strings.TrimSpace(s.SellerFilter))
	if sf != "" && sf != "any" && l.Commercial != nil {
		isCommercial := *l.Commercial
		if sf == "private" && isCommercial {
			return RejectSeller
		}
		if (sf == "commercial" || sf == "dealer" || sf == "dealership") && !isCommercial {
			return RejectSeller
		}
	}
	if s.PriceOnly && l.Price <= 0 {
		return RejectOtherFilter
	}
	if s.PhotoOnly && l.ImageURL == "" {
		return RejectOtherFilter
	}
	if s.Keywords != "" {
		desc := strings.ToLower(l.Description + " " + l.SubModel)
		for _, kw := range strings.Split(s.Keywords, ",") {
			kw = strings.TrimSpace(strings.ToLower(kw))
			if kw != "" && !strings.Contains(desc, kw) {
				return RejectOtherFilter
			}
		}
	}
	if s.ExcludeKeys != "" {
		desc := strings.ToLower(l.Description + " " + l.SubModel)
		for _, kw := range strings.Split(s.ExcludeKeys, ",") {
			kw = strings.TrimSpace(strings.ToLower(kw))
			if kw != "" && strings.Contains(desc, kw) {
				return RejectOtherFilter
			}
		}
	}
	return ""
}

// matchesSearch checks whether a single listing satisfies the filter
// criteria of a search.  The logic mirrors filter.Apply but operates on
// a storage.Search directly rather than on FilterCriteria, so the
// percolator is self-contained.
func matchesSearch(l model.RawListing, s storage.Search) bool {
	// Manufacturer / model identity.
	if s.Manufacturer > 0 && l.ManufacturerID != s.Manufacturer {
		return false
	}
	if s.Model > 0 && l.ModelID != s.Model {
		return false
	}

	// Year range.
	if s.YearMin > 0 && l.Year < s.YearMin {
		return false
	}
	if s.YearMax > 0 && l.Year > s.YearMax {
		return false
	}

	// Price range.
	if s.PriceMin > 0 && l.Price > 0 && l.Price < s.PriceMin {
		return false
	}
	if s.PriceMax > 0 && l.Price > s.PriceMax {
		return false
	}

	// Mileage cap.
	if s.MaxKm > 0 && l.Km > 0 && l.Km > s.MaxKm {
		return false
	}

	// Hand (ownership) cap.
	if s.MaxHand > 0 && l.Hand > s.MaxHand {
		return false
	}

	// Engine size minimum (matches filter.Apply behavior).
	if s.EngineMinCC > 0 && int(l.EngineVolume) < s.EngineMinCC {
		return false
	}

	// Gearbox filter.
	if s.GearBox != "" && l.GearBox != "" {
		if !strings.EqualFold(s.GearBox, l.GearBox) {
			return false
		}
	}

	// Seller filter (private / commercial / dealer).
	sf := strings.ToLower(strings.TrimSpace(s.SellerFilter))
	if sf != "" && sf != "any" && l.Commercial != nil {
		isCommercial := *l.Commercial
		if sf == "private" && isCommercial {
			return false
		}
		if (sf == "commercial" || sf == "dealer" || sf == "dealership") && !isCommercial {
			return false
		}
	}

	// Price-only filter.
	if s.PriceOnly && l.Price <= 0 {
		return false
	}

	// Photo-only filter.
	if s.PhotoOnly && l.ImageURL == "" {
		return false
	}

	// Include keywords.
	if s.Keywords != "" {
		desc := strings.ToLower(l.Description + " " + l.SubModel)
		for _, kw := range strings.Split(s.Keywords, ",") {
			kw = strings.TrimSpace(strings.ToLower(kw))
			if kw != "" && !strings.Contains(desc, kw) {
				return false
			}
		}
	}

	// Exclude keywords.
	if s.ExcludeKeys != "" {
		desc := strings.ToLower(l.Description + " " + l.SubModel)
		for _, kw := range strings.Split(s.ExcludeKeys, ",") {
			kw = strings.TrimSpace(strings.ToLower(kw))
			if kw != "" && strings.Contains(desc, kw) {
				return false
			}
		}
	}

	return true
}

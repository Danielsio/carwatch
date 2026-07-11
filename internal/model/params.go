package model

import (
	"strings"

	"github.com/dsionov/carwatch/internal/storage"
)

// FilterCriteriaFromSearch converts a storage.Search into the FilterCriteria
// used by the post-fetch filter pipeline. Both the API refresh handler and the
// scheduler were carrying identical copies of this logic; this single function
// replaces both.
func FilterCriteriaFromSearch(s *storage.Search) FilterCriteria {
	criteria := FilterCriteria{
		ManufacturerID: s.Manufacturer,
		ModelID:        s.Model,
		YearMin:        s.YearMin,
		YearMax:        s.YearMax,
		PriceMin:       s.PriceMin,
		PriceMax:       s.PriceMax,
		EngineMinCC:    float64(s.EngineMinCC),
		MaxKm:          s.MaxKm,
		MaxHand:        s.MaxHand,
		GearBox:        s.GearBox,
		SellerFilter:   s.SellerFilter,
		PriceOnly:      s.PriceOnly,
		PhotoOnly:      s.PhotoOnly,
	}

	if s.Keywords != "" {
		for _, kw := range strings.Split(s.Keywords, ",") {
			if kw = strings.TrimSpace(kw); kw != "" {
				criteria.Keywords = append(criteria.Keywords, kw)
			}
		}
	}
	if s.ExcludeKeys != "" {
		for _, ek := range strings.Split(s.ExcludeKeys, ",") {
			if ek = strings.TrimSpace(ek); ek != "" {
				criteria.ExcludeKeys = append(criteria.ExcludeKeys, ek)
			}
		}
	}

	return criteria
}

// SourceParamsFromSearch builds the SourceParams sent to listing sources
// from a saved search definition. Used by both the scheduler's targeted
// fetches and the API refresh handler.
func SourceParamsFromSearch(s *storage.Search) SourceParams {
	return SourceParams{
		Manufacturer: s.Manufacturer,
		Model:        s.Model,
		YearMin:      s.YearMin,
		YearMax:      s.YearMax,
		PriceMin:     s.PriceMin,
		PriceMax:     s.PriceMax,
		MaxKm:        s.MaxKm,
		MaxHand:      s.MaxHand,
		EngineMinCC:  s.EngineMinCC,
		SellerFilter: s.SellerFilter,
		PhotoOnly:    s.PhotoOnly,
	}
}

// SourceParams defines the search parameters sent to listing sources.
type SourceParams struct {
	Manufacturer int
	Model        int
	YearMin      int
	YearMax      int
	PriceMin     int
	PriceMax     int
	Page         int
	MaxKm        int
	MaxHand      int
	EngineMinCC  int
	SellerFilter string
	PhotoOnly    bool
}

// FilterCriteria defines the criteria used to filter raw listings
// after they are fetched from a source.
type FilterCriteria struct {
	ManufacturerID int
	ModelID        int
	YearMin        int
	YearMax        int
	PriceMin       int
	PriceMax       int
	EngineMinCC    float64
	EngineMaxCC    float64
	MaxKm          int
	MaxHand        int
	Keywords       []string
	ExcludeKeys    []string
	GearBox        string
	SellerFilter   string
	PriceOnly      bool
	PhotoOnly      bool
}

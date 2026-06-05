package filter

import (
	"strings"

	"github.com/dsionov/carwatch/internal/model"
)

func Apply(criteria model.FilterCriteria, listings []model.RawListing) []model.RawListing {
	keywords := make([]string, len(criteria.Keywords))
	for i, kw := range criteria.Keywords {
		keywords[i] = strings.ToLower(kw)
	}
	excludeKeys := make([]string, len(criteria.ExcludeKeys))
	for i, ex := range criteria.ExcludeKeys {
		excludeKeys[i] = strings.ToLower(ex)
	}
	criteria.Keywords = keywords
	criteria.ExcludeKeys = excludeKeys
	result := make([]model.RawListing, 0, len(listings))
	for _, l := range listings {
		if matches(criteria, l) {
			result = append(result, l)
		}
	}
	return result
}

func matches(c model.FilterCriteria, l model.RawListing) bool {
	if c.ModelID > 0 && l.ModelID != c.ModelID {
		return false
	}
	if c.PriceMin > 0 && l.Price > 0 && l.Price < c.PriceMin {
		return false
	}
	if c.PriceMax > 0 && l.Price > c.PriceMax {
		return false
	}
	if c.YearMin > 0 && l.Year < c.YearMin {
		return false
	}
	if c.YearMax > 0 && l.Year > c.YearMax {
		return false
	}
	if c.EngineMinCC > 0 && l.EngineVolume < c.EngineMinCC {
		return false
	}
	if c.EngineMaxCC > 0 && l.EngineVolume > c.EngineMaxCC {
		return false
	}
	if c.MaxKm > 0 && l.Km > 0 && l.Km > c.MaxKm {
		return false
	}
	if c.MaxHand > 0 && l.Hand > c.MaxHand {
		return false
	}
	if c.PriceOnly && l.Price <= 0 {
		return false
	}
	if c.PhotoOnly && l.ImageURL == "" {
		return false
	}
	if c.GearBox != "" && l.GearBox != "" && !strings.EqualFold(l.GearBox, c.GearBox) {
		return false
	}

	sf := strings.ToLower(strings.TrimSpace(c.SellerFilter))
	if sf != "" && sf != "any" && l.Commercial != nil {
		isCommercial := *l.Commercial
		if sf == "private" && isCommercial {
			return false
		}
		if (sf == "commercial" || sf == "dealer" || sf == "dealership") && !isCommercial {
			return false
		}
	}

	desc := strings.ToLower(l.Description + " " + l.SubModel)

	for _, kw := range c.Keywords {
		if !strings.Contains(desc, kw) {
			return false
		}
	}
	for _, ex := range c.ExcludeKeys {
		if strings.Contains(desc, ex) {
			return false
		}
	}

	return true
}

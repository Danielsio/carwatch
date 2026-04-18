package filter

import (
	"strings"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

func Apply(criteria config.FilterCriteria, listings []model.RawListing) []model.RawListing {
	result := make([]model.RawListing, 0, len(listings))
	for _, l := range listings {
		if matches(criteria, l) {
			result = append(result, l)
		}
	}
	return result
}

func matches(c config.FilterCriteria, l model.RawListing) bool {
	if c.EngineMin > 0 && l.EngineVolume < c.EngineMin {
		return false
	}
	if c.EngineMax > 0 && l.EngineVolume > c.EngineMax {
		return false
	}
	if c.MaxKm > 0 && l.Km > c.MaxKm {
		return false
	}
	if c.MaxHand > 0 && l.Hand > c.MaxHand {
		return false
	}

	desc := strings.ToLower(l.Description)

	for _, kw := range c.Keywords {
		if !strings.Contains(desc, strings.ToLower(kw)) {
			return false
		}
	}
	for _, ex := range c.ExcludeKeys {
		if strings.Contains(desc, strings.ToLower(ex)) {
			return false
		}
	}

	return true
}

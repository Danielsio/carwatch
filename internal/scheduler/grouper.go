package scheduler

import (
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
)

type CanonicalGroup struct {
	Manufacturer int
	Model        int
	Params       config.SourceParams
	Searches     []storage.Search
}

func GroupSearches(searches []storage.Search) []CanonicalGroup {
	type groupKey struct {
		Manufacturer int
		Model        int
	}

	grouped := make(map[groupKey]*CanonicalGroup)

	for _, s := range searches {
		key := groupKey{s.Manufacturer, s.Model}
		g, ok := grouped[key]
		if !ok {
			g = &CanonicalGroup{
				Manufacturer: s.Manufacturer,
				Model:        s.Model,
				Params: config.SourceParams{
					Manufacturer: s.Manufacturer,
					Model:        s.Model,
					YearMin:      s.YearMin,
					YearMax:      s.YearMax,
					PriceMax:     s.PriceMax,
				},
			}
			grouped[key] = g
		}

		if s.YearMin < g.Params.YearMin {
			g.Params.YearMin = s.YearMin
		}
		if s.YearMax > g.Params.YearMax {
			g.Params.YearMax = s.YearMax
		}
		if s.PriceMax > g.Params.PriceMax {
			g.Params.PriceMax = s.PriceMax
		}

		g.Searches = append(g.Searches, s)
	}

	groups := make([]CanonicalGroup, 0, len(grouped))
	for _, g := range grouped {
		groups = append(groups, *g)
	}
	return groups
}

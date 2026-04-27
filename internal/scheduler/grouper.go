package scheduler

import (
	"strings"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type CanonicalGroup struct {
	Source       string
	Manufacturer int
	Model        int
	Params       model.SourceParams
	Searches     []storage.Search
}

func GroupSearches(searches []storage.Search) []CanonicalGroup {
	type groupKey struct {
		Source       string
		Manufacturer int
		Model        int
	}

	grouped := make(map[groupKey]*CanonicalGroup)

	for _, s := range searches {
		for _, source := range splitSources(s.Source) {
			key := groupKey{source, s.Manufacturer, s.Model}
			g, ok := grouped[key]
			if !ok {
				g = &CanonicalGroup{
					Source:       source,
					Manufacturer: s.Manufacturer,
					Model:        s.Model,
					Params: model.SourceParams{
						Manufacturer: s.Manufacturer,
						Model:        s.Model,
						YearMin:      s.YearMin,
						YearMax:      s.YearMax,
						PriceMax:     s.PriceMax,
					},
				}
				grouped[key] = g
			}

			if g.Params.YearMin == 0 || (s.YearMin > 0 && s.YearMin < g.Params.YearMin) {
				g.Params.YearMin = s.YearMin
			}
			if g.Params.YearMax == 0 || s.YearMax > g.Params.YearMax {
				g.Params.YearMax = s.YearMax
			}
			if s.PriceMax == 0 || g.Params.PriceMax == 0 {
				g.Params.PriceMax = 0
			} else if s.PriceMax > g.Params.PriceMax {
				g.Params.PriceMax = s.PriceMax
			}

			g.Searches = append(g.Searches, s)
		}
	}

	groups := make([]CanonicalGroup, 0, len(grouped))
	for _, g := range grouped {
		groups = append(groups, *g)
	}
	return groups
}

func splitSources(source string) []string {
	if source == "" {
		return []string{"yad2", "winwin"}
	}
	parts := strings.Split(source, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return []string{"yad2", "winwin"}
	}
	return result
}

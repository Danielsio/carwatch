package catalog

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type Entry struct {
	ID   int
	Name string
}

type Catalog interface {
	Manufacturers() []Entry
	Models(manufacturerID int) []Entry
	ManufacturerName(id int) string
	ModelName(manufacturerID, modelID int) string
	SearchManufacturers(query string) []Entry
	SearchModels(manufacturerID int, query string) []Entry
}

func fuzzyMatch(name, query string) bool {
	return strings.Contains(stripDiacritics(strings.ToLower(name)), stripDiacritics(strings.ToLower(query)))
}

func stripDiacritics(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func searchEntries(entries []Entry, query string) []Entry {
	var results []Entry
	for _, e := range entries {
		if fuzzyMatch(e.Name, query) {
			results = append(results, e)
		}
	}
	return results
}

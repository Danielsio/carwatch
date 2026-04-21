package catalog

import "strings"

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
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
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

package catalog

import (
	_ "embed"
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"strings"
)

//go:embed catalog_data.json
var catalogDataJSON []byte

type catalogData struct {
	Manufacturers []catalogJSONEntry            `json:"manufacturers"`
	Models        map[string][]catalogJSONEntry `json:"models"`
}

type catalogJSONEntry struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameHe string `json:"name_he"`
}

var (
	defaultManufacturers []Entry
	defaultModels        map[int][]Entry
)

func init() {
	var data catalogData
	if err := json.Unmarshal(catalogDataJSON, &data); err != nil {
		log.Fatalf("catalog: failed to parse embedded catalog_data.json: %v", err)
	}

	defaultManufacturers = make([]Entry, 0, len(data.Manufacturers))
	for _, m := range data.Manufacturers {
		defaultManufacturers = append(defaultManufacturers, Entry{
			ID:     m.ID,
			Name:   m.Name,
			NameHe: m.NameHe,
		})
	}
	sort.Slice(defaultManufacturers, func(i, j int) bool {
		return strings.ToLower(defaultManufacturers[i].Name) < strings.ToLower(defaultManufacturers[j].Name)
	})

	defaultModels = make(map[int][]Entry, len(data.Models))
	for mfrIDStr, models := range data.Models {
		mfrID, _ := strconv.Atoi(mfrIDStr)
		entries := make([]Entry, 0, len(models))
		for _, m := range models {
			entries = append(entries, Entry{
				ID:     m.ID,
				Name:   m.Name,
				NameHe: m.NameHe,
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		})
		defaultModels[mfrID] = entries
	}
}

type staticCatalog struct {
	mfrs   []Entry
	models map[int][]Entry
}

func NewStatic() Catalog {
	return &staticCatalog{
		mfrs:   defaultManufacturers,
		models: defaultModels,
	}
}

func (c *staticCatalog) Manufacturers() []Entry {
	return c.mfrs
}

func (c *staticCatalog) Models(manufacturerID int) []Entry {
	return c.models[manufacturerID]
}

func (c *staticCatalog) ManufacturerName(id int) string {
	for _, m := range c.mfrs {
		if m.ID == id {
			return m.Name
		}
	}
	return "Unknown"
}

func (c *staticCatalog) ModelName(manufacturerID, modelID int) string {
	for _, m := range c.models[manufacturerID] {
		if m.ID == modelID {
			return m.Name
		}
	}
	return "Unknown"
}

func (c *staticCatalog) SearchManufacturers(query string) []Entry {
	return searchEntries(c.mfrs, query)
}

func (c *staticCatalog) SearchModels(manufacturerID int, query string) []Entry {
	return searchEntries(c.models[manufacturerID], query)
}

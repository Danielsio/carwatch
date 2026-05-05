package catalog

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

type DynamicCatalog struct {
	mu         sync.RWMutex
	mfrs       []Entry
	models     map[int][]Entry
	mfrMap     map[int]string
	mfrHeMap   map[int]string
	modelMap   map[int]map[int]string
	modelHeMap map[int]map[int]string
	fallback   Catalog
	logger     *slog.Logger
}

func NewDynamic(logger *slog.Logger) *DynamicCatalog {
	return &DynamicCatalog{
		models:     make(map[int][]Entry),
		mfrMap:     make(map[int]string),
		mfrHeMap:   make(map[int]string),
		modelMap:   make(map[int]map[int]string),
		modelHeMap: make(map[int]map[int]string),
		fallback:   NewStatic(),
		logger:     logger,
	}
}

func (d *DynamicCatalog) Load(_ context.Context) {
	d.logger.Info("seeding catalog from static fallback")
	d.mu.Lock()
	for _, m := range d.fallback.Manufacturers() {
		d.mfrMap[m.ID] = m.Name
		if m.NameHe != "" {
			d.mfrHeMap[m.ID] = m.NameHe
		}
		if d.modelMap[m.ID] == nil {
			d.modelMap[m.ID] = make(map[int]string)
		}
		if d.modelHeMap[m.ID] == nil {
			d.modelHeMap[m.ID] = make(map[int]string)
		}
		for _, mdl := range d.fallback.Models(m.ID) {
			d.modelMap[m.ID][mdl.ID] = mdl.Name
			if mdl.NameHe != "" {
				d.modelHeMap[m.ID][mdl.ID] = mdl.NameHe
			}
		}
	}
	d.rebuildSlices()
	d.mu.Unlock()
}

type IngestEntry struct {
	ManufacturerID     int
	ManufacturerName   string
	ManufacturerNameHe string
	ModelID            int
	ModelName          string
	ModelNameHe        string
}

func (d *DynamicCatalog) Ingest(_ context.Context, e IngestEntry) {
	if e.ManufacturerID == 0 || e.ManufacturerName == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	changed := false

	if _, ok := d.mfrMap[e.ManufacturerID]; !ok {
		d.mfrMap[e.ManufacturerID] = e.ManufacturerName
		changed = true
	}
	if e.ManufacturerNameHe != "" {
		if _, ok := d.mfrHeMap[e.ManufacturerID]; !ok {
			d.mfrHeMap[e.ManufacturerID] = e.ManufacturerNameHe
			changed = true
		}
	}

	if e.ModelID != 0 && e.ModelName != "" {
		if d.modelMap[e.ManufacturerID] == nil {
			d.modelMap[e.ManufacturerID] = make(map[int]string)
		}
		if _, ok := d.modelMap[e.ManufacturerID][e.ModelID]; !ok {
			d.modelMap[e.ManufacturerID][e.ModelID] = e.ModelName
			changed = true
		}
		if e.ModelNameHe != "" {
			if d.modelHeMap[e.ManufacturerID] == nil {
				d.modelHeMap[e.ManufacturerID] = make(map[int]string)
			}
			if _, ok := d.modelHeMap[e.ManufacturerID][e.ModelID]; !ok {
				d.modelHeMap[e.ManufacturerID][e.ModelID] = e.ModelNameHe
				changed = true
			}
		}
	}

	if changed {
		d.rebuildSlices()
	}
}

func (d *DynamicCatalog) Flush(_ context.Context) {
	// No-op: catalog is now purely in-memory, seeded from static data
	// and enriched via Ingest() during each scrape cycle.
}

func (d *DynamicCatalog) rebuildSlices() {
	mfrs := make([]Entry, 0, len(d.mfrMap))
	for id, name := range d.mfrMap {
		mfrs = append(mfrs, Entry{ID: id, Name: name, NameHe: d.mfrHeMap[id]})
	}
	sort.Slice(mfrs, func(i, j int) bool {
		li, lj := strings.ToLower(mfrs[i].Name), strings.ToLower(mfrs[j].Name)
		if li == lj {
			return mfrs[i].ID < mfrs[j].ID
		}
		return li < lj
	})
	d.mfrs = mfrs

	models := make(map[int][]Entry, len(d.modelMap))
	for mfrID, mdls := range d.modelMap {
		heMap := d.modelHeMap[mfrID]
		list := make([]Entry, 0, len(mdls))
		for id, name := range mdls {
			he := ""
			if heMap != nil {
				he = heMap[id]
			}
			list = append(list, Entry{ID: id, Name: name, NameHe: he})
		}
		sort.Slice(list, func(i, j int) bool {
			li, lj := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
			if li == lj {
				return list[i].ID < list[j].ID
			}
			return li < lj
		})
		models[mfrID] = list
	}
	d.models = models
}

func (d *DynamicCatalog) Manufacturers() []Entry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]Entry, len(d.mfrs))
	copy(result, d.mfrs)
	return result
}

func (d *DynamicCatalog) Models(manufacturerID int) []Entry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	src := d.models[manufacturerID]
	if src == nil {
		return nil
	}
	result := make([]Entry, len(src))
	copy(result, src)
	return result
}

func (d *DynamicCatalog) ManufacturerName(id int) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if name, ok := d.mfrMap[id]; ok {
		return name
	}
	return "Unknown"
}

func (d *DynamicCatalog) ModelName(manufacturerID, modelID int) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if mdls, ok := d.modelMap[manufacturerID]; ok {
		if name, ok := mdls[modelID]; ok {
			return name
		}
	}
	return "Unknown"
}

func (d *DynamicCatalog) SearchManufacturers(query string) []Entry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return searchEntries(d.mfrs, query)
}

func (d *DynamicCatalog) SearchModels(manufacturerID int, query string) []Entry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return searchEntries(d.models[manufacturerID], query)
}

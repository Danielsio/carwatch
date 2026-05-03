package catalog

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

type DynamicCatalog struct {
	mu       sync.RWMutex
	mfrs     []Entry
	models   map[int][]Entry
	mfrMap   map[int]string
	modelMap map[int]map[int]string
	fallback Catalog
	logger   *slog.Logger
}

func NewDynamic(logger *slog.Logger) *DynamicCatalog {
	return &DynamicCatalog{
		models:   make(map[int][]Entry),
		mfrMap:   make(map[int]string),
		modelMap: make(map[int]map[int]string),
		fallback: NewStatic(),
		logger:   logger,
	}
}

func (d *DynamicCatalog) Load(_ context.Context) {
	d.logger.Info("seeding catalog from static fallback")
	d.mu.Lock()
	for _, m := range d.fallback.Manufacturers() {
		d.mfrMap[m.ID] = m.Name
		if d.modelMap[m.ID] == nil {
			d.modelMap[m.ID] = make(map[int]string)
		}
		for _, mdl := range d.fallback.Models(m.ID) {
			d.modelMap[m.ID][mdl.ID] = mdl.Name
		}
	}
	d.rebuildSlices()
	d.mu.Unlock()
}

func (d *DynamicCatalog) Ingest(_ context.Context, manufacturerID int, manufacturerName string, modelID int, modelName string) {
	if manufacturerID == 0 || manufacturerName == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	changed := false

	if _, ok := d.mfrMap[manufacturerID]; !ok {
		d.mfrMap[manufacturerID] = manufacturerName
		changed = true
	}

	if modelID != 0 && modelName != "" {
		if d.modelMap[manufacturerID] == nil {
			d.modelMap[manufacturerID] = make(map[int]string)
		}
		if _, ok := d.modelMap[manufacturerID][modelID]; !ok {
			d.modelMap[manufacturerID][modelID] = modelName
			changed = true
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
		mfrs = append(mfrs, Entry{ID: id, Name: name})
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
		list := make([]Entry, 0, len(mdls))
		for id, name := range mdls {
			list = append(list, Entry{ID: id, Name: name})
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
	return d.mfrs
}

func (d *DynamicCatalog) Models(manufacturerID int) []Entry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.models[manufacturerID]
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

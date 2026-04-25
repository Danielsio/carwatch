package catalog

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

const saveCooldown = 5 * time.Minute

type DynamicCatalog struct {
	mu        sync.RWMutex
	mfrs      []Entry
	models    map[int][]Entry
	mfrMap    map[int]string
	modelMap  map[int]map[int]string
	dirty     bool
	lastSave  time.Time
	saveGen   int64
	saveMu    sync.Mutex
	store     storage.CatalogStore
	fallback  Catalog
	logger    *slog.Logger
}

func NewDynamic(store storage.CatalogStore, logger *slog.Logger) *DynamicCatalog {
	return &DynamicCatalog{
		models:   make(map[int][]Entry),
		mfrMap:   make(map[int]string),
		modelMap: make(map[int]map[int]string),
		store:    store,
		fallback: NewStatic(),
		logger:   logger,
	}
}

func (d *DynamicCatalog) Load(ctx context.Context) {
	if d.store != nil {
		if d.loadFromStore(ctx) {
			d.logger.Info("catalog loaded from cache",
				"manufacturers", len(d.mfrs))
			return
		}
	}

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

func (d *DynamicCatalog) Ingest(ctx context.Context, manufacturerID int, manufacturerName string, modelID int, modelName string) {
	if manufacturerID == 0 || manufacturerName == "" {
		return
	}

	entries, gen, shouldSave := d.ingestLocked(manufacturerID, manufacturerName, modelID, modelName)
	if shouldSave {
		d.persistEntries(ctx, entries, gen)
	}
}

func (d *DynamicCatalog) ingestLocked(manufacturerID int, manufacturerName string, modelID int, modelName string) ([]storage.CatalogEntry, int64, bool) {
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

	if !changed {
		return nil, 0, false
	}

	d.rebuildSlices()
	d.dirty = true
	d.saveGen++
	gen := d.saveGen

	shouldSave := d.store != nil && time.Since(d.lastSave) > saveCooldown
	var entries []storage.CatalogEntry
	if shouldSave {
		entries = d.buildEntries()
	}
	return entries, gen, shouldSave
}

func (d *DynamicCatalog) Flush(ctx context.Context) {
	d.mu.Lock()
	if !d.dirty || d.store == nil {
		d.mu.Unlock()
		return
	}
	entries := d.buildEntries()
	d.saveGen++
	gen := d.saveGen
	d.mu.Unlock()

	d.persistEntries(ctx, entries, gen)
}

func (d *DynamicCatalog) rebuildSlices() {
	mfrs := make([]Entry, 0, len(d.mfrMap))
	for id, name := range d.mfrMap {
		mfrs = append(mfrs, Entry{ID: id, Name: name})
	}
	sort.Slice(mfrs, func(i, j int) bool { return mfrs[i].Name < mfrs[j].Name })
	d.mfrs = mfrs

	models := make(map[int][]Entry, len(d.modelMap))
	for mfrID, mdls := range d.modelMap {
		list := make([]Entry, 0, len(mdls))
		for id, name := range mdls {
			list = append(list, Entry{ID: id, Name: name})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		models[mfrID] = list
	}
	d.models = models
}

func (d *DynamicCatalog) buildEntries() []storage.CatalogEntry {
	var entries []storage.CatalogEntry
	for mfrID, mfrName := range d.mfrMap {
		mdls := d.modelMap[mfrID]
		if len(mdls) == 0 {
			entries = append(entries, storage.CatalogEntry{
				ManufacturerID:   mfrID,
				ManufacturerName: mfrName,
			})
			continue
		}
		for mdlID, mdlName := range mdls {
			entries = append(entries, storage.CatalogEntry{
				ManufacturerID:   mfrID,
				ManufacturerName: mfrName,
				ModelID:          mdlID,
				ModelName:        mdlName,
			})
		}
	}
	return entries
}

func (d *DynamicCatalog) persistEntries(ctx context.Context, entries []storage.CatalogEntry, gen int64) {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()

	d.mu.RLock()
	stale := gen != d.saveGen
	d.mu.RUnlock()
	if stale {
		return
	}

	if err := d.store.SaveCatalogEntries(ctx, entries); err != nil {
		d.logger.Error("catalog save failed", "error", err)
		return
	}
	d.mu.Lock()
	if d.saveGen == gen {
		d.dirty = false
	}
	d.lastSave = time.Now()
	d.mu.Unlock()
	d.logger.Info("catalog saved", "entries", len(entries))
}

func (d *DynamicCatalog) loadFromStore(ctx context.Context) bool {
	entries, err := d.store.LoadCatalogEntries(ctx)
	if err != nil || len(entries) == 0 {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, e := range entries {
		d.mfrMap[e.ManufacturerID] = e.ManufacturerName
		if d.modelMap[e.ManufacturerID] == nil {
			d.modelMap[e.ManufacturerID] = make(map[int]string)
		}
		if e.ModelID != 0 {
			d.modelMap[e.ManufacturerID][e.ModelID] = e.ModelName
		}
	}

	// Merge static fallback entries that might be missing from cache
	for _, m := range d.fallback.Manufacturers() {
		if _, ok := d.mfrMap[m.ID]; !ok {
			d.mfrMap[m.ID] = m.Name
		}
		if d.modelMap[m.ID] == nil {
			d.modelMap[m.ID] = make(map[int]string)
		}
		for _, mdl := range d.fallback.Models(m.ID) {
			if _, ok := d.modelMap[m.ID][mdl.ID]; !ok {
				d.modelMap[m.ID][mdl.ID] = mdl.Name
			}
		}
	}

	d.rebuildSlices()
	return true
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

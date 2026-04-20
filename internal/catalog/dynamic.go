package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/storage"
)

const (
	cacheTTL    = 24 * time.Hour
	fetchPages  = 5
	baseURL     = "https://www.yad2.co.il/vehicles/cars"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type DynamicCatalog struct {
	mu       sync.RWMutex
	mfrs     []Entry
	models   map[int][]Entry
	store    storage.CatalogStore
	client   httpClient
	fallback Catalog
	logger   *slog.Logger
}

func NewDynamic(store storage.CatalogStore, client httpClient, logger *slog.Logger) *DynamicCatalog {
	d := &DynamicCatalog{
		models:   make(map[int][]Entry),
		store:    store,
		client:   client,
		fallback: NewStatic(),
		logger:   logger,
	}
	return d
}

func (d *DynamicCatalog) Load(ctx context.Context) {
	age, err := d.store.CatalogAge(ctx)
	if err == nil && age < cacheTTL {
		if d.loadFromStore(ctx) {
			d.logger.Info("catalog loaded from cache", "age", age.Round(time.Minute))
			return
		}
	}

	if d.refresh(ctx) {
		return
	}

	if d.loadFromStore(ctx) {
		d.logger.Warn("using stale catalog cache")
		return
	}

	d.logger.Warn("using static fallback catalog")
	d.mu.Lock()
	d.mfrs = d.fallback.Manufacturers()
	for _, m := range d.mfrs {
		d.models[m.ID] = d.fallback.Models(m.ID)
	}
	d.mu.Unlock()
}

func (d *DynamicCatalog) StartRefreshLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cacheTTL)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.refresh(ctx)
			}
		}
	}()
}

func (d *DynamicCatalog) refresh(ctx context.Context) bool {
	items, err := d.fetchCatalog(ctx)
	if err != nil {
		d.logger.Error("catalog fetch failed", "error", err)
		return false
	}
	if len(items) == 0 {
		d.logger.Warn("catalog fetch returned no items")
		return false
	}

	mfrs, models := d.buildCatalog(items)
	if len(mfrs) == 0 {
		return false
	}

	var storeEntries []storage.CatalogEntry
	for _, m := range mfrs {
		for _, mdl := range models[m.ID] {
			storeEntries = append(storeEntries, storage.CatalogEntry{
				ManufacturerID:   m.ID,
				ManufacturerName: m.Name,
				ModelID:          mdl.ID,
				ModelName:        mdl.Name,
			})
		}
	}
	if err := d.store.SaveCatalogEntries(ctx, storeEntries); err != nil {
		d.logger.Error("catalog save failed", "error", err)
	}

	d.mu.Lock()
	d.mfrs = mfrs
	d.models = models
	d.mu.Unlock()

	d.logger.Info("catalog refreshed", "manufacturers", len(mfrs), "total_models", len(storeEntries))
	return true
}

func (d *DynamicCatalog) loadFromStore(ctx context.Context) bool {
	entries, err := d.store.LoadCatalogEntries(ctx)
	if err != nil || len(entries) == 0 {
		return false
	}

	mfrMap := make(map[int]string)
	modelsMap := make(map[int]map[int]string)

	for _, e := range entries {
		mfrMap[e.ManufacturerID] = e.ManufacturerName
		if modelsMap[e.ManufacturerID] == nil {
			modelsMap[e.ManufacturerID] = make(map[int]string)
		}
		modelsMap[e.ManufacturerID][e.ModelID] = e.ModelName
	}

	mfrs := make([]Entry, 0, len(mfrMap))
	for id, name := range mfrMap {
		mfrs = append(mfrs, Entry{ID: id, Name: name})
	}
	sort.Slice(mfrs, func(i, j int) bool { return mfrs[i].Name < mfrs[j].Name })

	models := make(map[int][]Entry)
	for mfrID, mdls := range modelsMap {
		list := make([]Entry, 0, len(mdls))
		for id, name := range mdls {
			list = append(list, Entry{ID: id, Name: name})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		models[mfrID] = list
	}

	d.mu.Lock()
	d.mfrs = mfrs
	d.models = models
	d.mu.Unlock()
	return true
}

func (d *DynamicCatalog) fetchCatalog(ctx context.Context) ([]yad2.CatalogItem, error) {
	var all []yad2.CatalogItem

	for page := 1; page <= fetchPages; page++ {
		url := fmt.Sprintf("%s?page=%d&Order=1", baseURL, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("Accept-Language", "he-IL,he;q=0.9,en;q=0.8")

		resp, err := d.client.Do(req)
		if err != nil {
			d.logger.Warn("catalog page fetch failed", "page", page, "error", err)
			continue
		}

		items, err := yad2.ParseCatalogFromPage(resp.Body)
		resp.Body.Close()
		if err != nil {
			d.logger.Warn("catalog page parse failed", "page", page, "error", err)
			continue
		}
		all = append(all, items...)

		if page < fetchPages {
			time.Sleep(2 * time.Second)
		}
	}
	return all, nil
}

func (d *DynamicCatalog) buildCatalog(items []yad2.CatalogItem) ([]Entry, map[int][]Entry) {
	mfrMap := make(map[int]string)
	modelMap := make(map[int]map[int]string)

	// Merge with static fallback first
	for _, m := range d.fallback.Manufacturers() {
		mfrMap[m.ID] = m.Name
		if modelMap[m.ID] == nil {
			modelMap[m.ID] = make(map[int]string)
		}
		for _, mdl := range d.fallback.Models(m.ID) {
			modelMap[m.ID][mdl.ID] = mdl.Name
		}
	}

	for _, item := range items {
		if item.ManufacturerID == 0 || item.ManufacturerName == "" {
			continue
		}
		name := item.ManufacturerName
		if existing, ok := mfrMap[item.ManufacturerID]; ok && existing != "" {
			name = existing
		}
		mfrMap[item.ManufacturerID] = name

		if item.ModelID == 0 || item.ModelName == "" {
			continue
		}
		if modelMap[item.ManufacturerID] == nil {
			modelMap[item.ManufacturerID] = make(map[int]string)
		}
		mdlName := item.ModelName
		if existing, ok := modelMap[item.ManufacturerID][item.ModelID]; ok && existing != "" {
			mdlName = existing
		}
		modelMap[item.ManufacturerID][item.ModelID] = mdlName
	}

	mfrs := make([]Entry, 0, len(mfrMap))
	for id, name := range mfrMap {
		mfrs = append(mfrs, Entry{ID: id, Name: name})
	}
	sort.Slice(mfrs, func(i, j int) bool { return mfrs[i].Name < mfrs[j].Name })

	models := make(map[int][]Entry)
	for mfrID, mdls := range modelMap {
		list := make([]Entry, 0, len(mdls))
		for id, name := range mdls {
			list = append(list, Entry{ID: id, Name: name})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		models[mfrID] = list
	}

	return mfrs, models
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
	for _, m := range d.mfrs {
		if m.ID == id {
			return m.Name
		}
	}
	return "Unknown"
}

func (d *DynamicCatalog) ModelName(manufacturerID, modelID int) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, m := range d.models[manufacturerID] {
		if m.ID == modelID {
			return m.Name
		}
	}
	return "Unknown"
}


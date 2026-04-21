package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/storage"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// --- mock implementations ---

type mockCatalogStore struct {
	entries      []storage.CatalogEntry
	age          time.Duration
	ageErr       error
	loadErr      error
	saveErr      error
	saveCalled   bool
	savedEntries []storage.CatalogEntry
}

func (m *mockCatalogStore) SaveCatalogEntries(_ context.Context, entries []storage.CatalogEntry) error {
	m.saveCalled = true
	m.savedEntries = entries
	return m.saveErr
}

func (m *mockCatalogStore) LoadCatalogEntries(_ context.Context) ([]storage.CatalogEntry, error) {
	return m.entries, m.loadErr
}

func (m *mockCatalogStore) CatalogAge(_ context.Context) (time.Duration, error) {
	return m.age, m.ageErr
}

type mockHTTPClient struct {
	responses []*http.Response
	err       error
	callCount int
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	if len(m.responses) > 0 {
		idx := m.callCount - 1
		if idx >= len(m.responses) {
			idx = len(m.responses) - 1
		}
		return m.responses[idx], nil
	}
	return nil, errors.New("no response configured")
}

func makeYad2PageHTML(items string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"dehydratedState":{"queries":[{"state":{"data":{"data":{"feed":{"feed_items":[%s]}}}}}]}}}}
</script></body></html>`, items)
}

func makeFeedItem(mfrID int, mfrName string, modelID int, modelName string) string {
	return fmt.Sprintf(`{
		"token":"tok-%d-%d",
		"manufacturer":{"text":"%s","english_text":"%s","id":%d},
		"model":{"text":"%s","english_text":"%s","id":%d},
		"year_of_production":2023,
		"price":100000
	}`, mfrID, modelID, mfrName, mfrName, mfrID, modelName, modelName, modelID)
}

func makeHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// --- tests ---

func TestNewDynamic(t *testing.T) {
	store := &mockCatalogStore{}
	d := NewDynamic(store, nil, testLogger)

	if d.store != store {
		t.Error("store not set")
	}
	if d.models == nil {
		t.Error("models map should be initialized")
	}
	if d.fallback == nil {
		t.Error("fallback should be set")
	}
}

func TestDynamicCatalog_Load_FromFreshCache(t *testing.T) {
	store := &mockCatalogStore{
		age: 1 * time.Hour,
		entries: []storage.CatalogEntry{
			{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
			{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 101, ModelName: "A4"},
			{ManufacturerID: 2, ManufacturerName: "BMW", ModelID: 200, ModelName: "3 Series"},
		},
	}

	d := NewDynamic(store, nil, testLogger)
	d.Load(context.Background())

	mfrs := d.Manufacturers()
	if len(mfrs) != 2 {
		t.Fatalf("expected 2 manufacturers, got %d", len(mfrs))
	}
	if mfrs[0].Name != "Audi" {
		t.Errorf("first manufacturer = %q, want Audi (sorted)", mfrs[0].Name)
	}

	models := d.Models(1)
	if len(models) != 2 {
		t.Fatalf("expected 2 Audi models, got %d", len(models))
	}
}

func TestDynamicCatalog_Load_StaleCache_RefreshFails_UsesStaleCache(t *testing.T) {
	store := &mockCatalogStore{
		age: 48 * time.Hour,
		entries: []storage.CatalogEntry{
			{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
		},
	}
	client := &mockHTTPClient{err: errors.New("network error")}

	d := NewDynamic(store, client, testLogger)
	d.Load(context.Background())

	mfrs := d.Manufacturers()
	if len(mfrs) != 1 || mfrs[0].Name != "Audi" {
		t.Errorf("expected stale cache with Audi, got %v", mfrs)
	}
}

func TestDynamicCatalog_Load_NoCache_RefreshFails_UsesFallback(t *testing.T) {
	store := &mockCatalogStore{
		ageErr: errors.New("no cache"),
	}
	client := &mockHTTPClient{err: errors.New("network error")}

	d := NewDynamic(store, client, testLogger)
	d.Load(context.Background())

	mfrs := d.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected static fallback manufacturers (>10), got %d", len(mfrs))
	}
}

func TestDynamicCatalog_Load_RefreshSuccess(t *testing.T) {
	store := &mockCatalogStore{
		ageErr: errors.New("no cache"),
	}
	items := makeFeedItem(99, "TestMfr", 999, "TestModel")
	html := makeYad2PageHTML(items)
	responses := make([]*http.Response, fetchPages)
	for i := range responses {
		responses[i] = makeHTTPResponse(html)
	}
	client := &mockHTTPClient{responses: responses}

	d := NewDynamic(store, client, testLogger)
	d.Load(context.Background())

	if !store.saveCalled {
		t.Error("expected store.SaveCatalogEntries to be called")
	}

	if name := d.ManufacturerName(99); name == "Unknown" {
		t.Error("TestMfr should be found after refresh")
	}
}

func TestDynamicCatalog_Refresh_SaveError(t *testing.T) {
	store := &mockCatalogStore{
		ageErr:  errors.New("no cache"),
		saveErr: errors.New("db write failed"),
	}
	items := makeFeedItem(99, "TestMfr", 999, "TestModel")
	html := makeYad2PageHTML(items)
	responses := make([]*http.Response, fetchPages)
	for i := range responses {
		responses[i] = makeHTTPResponse(html)
	}
	client := &mockHTTPClient{responses: responses}

	d := NewDynamic(store, client, testLogger)
	d.Load(context.Background())

	if name := d.ManufacturerName(99); name == "Unknown" {
		t.Error("in-memory catalog should be updated even on save error")
	}
}

func TestDynamicCatalog_Refresh_EmptyResults(t *testing.T) {
	store := &mockCatalogStore{
		ageErr: errors.New("no cache"),
	}
	html := makeYad2PageHTML("")
	responses := make([]*http.Response, fetchPages)
	for i := range responses {
		responses[i] = makeHTTPResponse(html)
	}
	client := &mockHTTPClient{responses: responses}

	d := NewDynamic(store, client, testLogger)
	d.Load(context.Background())

	mfrs := d.Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("empty refresh should fall back to static, got %d manufacturers", len(mfrs))
	}
}

func TestDynamicCatalog_BuildCatalog_MergesWithStatic(t *testing.T) {
	d := NewDynamic(nil, nil, testLogger)

	items := []yad2.CatalogItem{
		{ManufacturerID: 99, ManufacturerName: "NewBrand", ModelID: 999, ModelName: "NewModel"},
	}
	mfrs, models := d.buildCatalog(items)

	foundStatic := false
	foundNew := false
	for _, m := range mfrs {
		if m.Name == "Toyota" {
			foundStatic = true
		}
		if m.Name == "NewBrand" {
			foundNew = true
		}
	}
	if !foundStatic {
		t.Error("static manufacturer Toyota not found after merge")
	}
	if !foundNew {
		t.Error("new manufacturer NewBrand not found after merge")
	}
	if len(models[99]) != 1 || models[99][0].Name != "NewModel" {
		t.Error("NewModel should be present for manufacturer 99")
	}
}

func TestDynamicCatalog_BuildCatalog_SkipsInvalidItems(t *testing.T) {
	d := NewDynamic(nil, nil, testLogger)

	items := []yad2.CatalogItem{
		{ManufacturerID: 0, ManufacturerName: "", ModelID: 1, ModelName: "X"},
		{ManufacturerID: 5, ManufacturerName: "Valid", ModelID: 0, ModelName: ""},
	}
	mfrs, models := d.buildCatalog(items)

	for _, m := range mfrs {
		if m.ID == 0 {
			t.Error("manufacturer with ID 0 should be skipped")
		}
	}
	for _, m := range models[5] {
		if m.ID == 0 {
			t.Error("model with ID 0 should be skipped")
		}
	}
}

func TestDynamicCatalog_BuildCatalog_PreservesStaticNames(t *testing.T) {
	d := NewDynamic(nil, nil, testLogger)

	items := []yad2.CatalogItem{
		{ManufacturerID: 19, ManufacturerName: "טויוטה", ModelID: 10226, ModelName: "קורולה"},
	}
	mfrs, _ := d.buildCatalog(items)

	for _, m := range mfrs {
		if m.ID == 19 {
			if m.Name != "Toyota" {
				t.Errorf("static name should be preserved, got %q", m.Name)
			}
			return
		}
	}
	t.Error("manufacturer 19 not found")
}

func TestDynamicCatalog_LoadFromStore(t *testing.T) {
	store := &mockCatalogStore{
		entries: []storage.CatalogEntry{
			{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
			{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 101, ModelName: "A4"},
		},
	}

	d := NewDynamic(store, nil, testLogger)
	ok := d.loadFromStore(context.Background())

	if !ok {
		t.Fatal("loadFromStore should return true")
	}
	if len(d.Manufacturers()) != 1 || d.Manufacturers()[0].Name != "Audi" {
		t.Error("manufacturers not loaded correctly")
	}
	if len(d.Models(1)) != 2 {
		t.Error("models not loaded correctly")
	}
}

func TestDynamicCatalog_LoadFromStore_Error(t *testing.T) {
	store := &mockCatalogStore{loadErr: errors.New("db error")}
	d := NewDynamic(store, nil, testLogger)
	if d.loadFromStore(context.Background()) {
		t.Error("loadFromStore should return false on error")
	}
}

func TestDynamicCatalog_LoadFromStore_Empty(t *testing.T) {
	store := &mockCatalogStore{entries: nil}
	d := NewDynamic(store, nil, testLogger)
	if d.loadFromStore(context.Background()) {
		t.Error("loadFromStore should return false when empty")
	}
}

func TestDynamicCatalog_Models_UnknownManufacturer(t *testing.T) {
	d := NewDynamic(nil, nil, testLogger)
	models := d.Models(99999)
	if models != nil {
		t.Errorf("expected nil for unknown manufacturer, got %v", models)
	}
}

func TestDynamicCatalog_ModelName(t *testing.T) {
	store := &mockCatalogStore{
		entries: []storage.CatalogEntry{
			{ManufacturerID: 1, ManufacturerName: "Audi", ModelID: 100, ModelName: "A3"},
		},
	}
	d := NewDynamic(store, nil, testLogger)
	d.loadFromStore(context.Background())

	if name := d.ModelName(1, 100); name != "A3" {
		t.Errorf("ModelName(1,100) = %q, want A3", name)
	}
	if name := d.ModelName(1, 999); name != "Unknown" {
		t.Errorf("ModelName(1,999) = %q, want Unknown", name)
	}
	if name := d.ModelName(999, 1); name != "Unknown" {
		t.Errorf("ModelName(999,1) = %q, want Unknown", name)
	}
}

func TestDynamicCatalog_StartRefreshLoop_ContextCancel(t *testing.T) {
	store := &mockCatalogStore{ageErr: errors.New("no cache")}
	d := NewDynamic(store, nil, testLogger)

	ctx, cancel := context.WithCancel(context.Background())
	d.StartRefreshLoop(ctx)
	cancel()
}

func TestDynamicCatalog_FetchCatalog_HTTPError(t *testing.T) {
	client := &mockHTTPClient{err: errors.New("connection refused")}
	d := NewDynamic(nil, client, testLogger)

	items, err := d.fetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("fetchCatalog should not return error on individual page failures, got: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items on all page failures, got %d", len(items))
	}
}

func TestDynamicCatalog_FetchCatalog_ParseError(t *testing.T) {
	responses := make([]*http.Response, fetchPages)
	for i := range responses {
		responses[i] = makeHTTPResponse("<html>no next data here</html>")
	}
	client := &mockHTTPClient{responses: responses}
	d := NewDynamic(nil, client, testLogger)

	items, err := d.fetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items on parse failure, got %d", len(items))
	}
}

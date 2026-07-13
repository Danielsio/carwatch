package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// schedulerStatusView mirrors the fields of schedulerStatusResponse the tests
// assert on.
type schedulerStatusView struct {
	LastCycleAt        *string `json:"last_cycle_at"`
	LastCycleStatus    string  `json:"last_cycle_status"`
	NextCycleAt        *string `json:"next_cycle_at"`
	PollingIntervalSec int     `json:"polling_interval_seconds"`
	Searches           int     `json:"searches"`
	ListingsFetched    int     `json:"listings_fetched"`
	ListingsMatched    int     `json:"listings_matched"`
	Notifications      int     `json:"notifications"`
}

func getSchedulerStatus(t *testing.T, srv *Server) schedulerStatusView {
	t.Helper()
	w := doRequest(t, srv, "GET", "/api/v1/scheduler/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("scheduler/status: %d %s", w.Code, w.Body.String())
	}
	var v schedulerStatusView
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	return v
}

func extIngestListing(token string) map[string]any {
	return map[string]any{
		"token":           token,
		"manufacturer":    "Toyota",
		"manufacturer_id": 19,
		"model":           "Corolla",
		"model_id":        10226,
		"year":            2021,
		"price":           95000,
		"km":              30000,
		"hand":            1,
		"city":            "תל אביב",
		"page_link":       "https://www.yad2.co.il/vehicles/item/" + token,
	}
}

// The core of the timer fix: an ingest push carrying the extension's cycle
// self-report must drive /scheduler/status, so the web UI counts down to the
// same Chrome alarm as the extension popup.
func TestIngestCycleReportDrivesSchedulerStatus(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/searches", map[string]any{
		"manufacturer": 19, "model": 10226,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create search: %d %s", w.Code, w.Body.String())
	}

	started := time.Now().UTC().Add(-90 * time.Second).Truncate(time.Second)
	next := time.Now().UTC().Add(13 * time.Minute).Truncate(time.Second)
	cycle := map[string]any{
		"started_at":   started.Format(time.RFC3339),
		"next_run_at":  next.Format(time.RFC3339),
		"interval_sec": 900,
	}

	w = doRequest(t, srv, "POST", "/api/v1/ext/ingest", map[string]any{
		"listings": []map[string]any{extIngestListing("ext-cycle-1")},
		"cycle":    cycle,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}

	st := getSchedulerStatus(t, srv)
	if st.NextCycleAt == nil || *st.NextCycleAt != next.Format(time.RFC3339) {
		t.Errorf("next_cycle_at = %v, want %s", st.NextCycleAt, next.Format(time.RFC3339))
	}
	if st.LastCycleAt == nil || *st.LastCycleAt != started.Format(time.RFC3339) {
		t.Errorf("last_cycle_at = %v, want %s", st.LastCycleAt, started.Format(time.RFC3339))
	}
	if st.PollingIntervalSec != 900 {
		t.Errorf("polling_interval_seconds = %d, want 900", st.PollingIntervalSec)
	}
	if st.LastCycleStatus != "ok" {
		t.Errorf("last_cycle_status = %q", st.LastCycleStatus)
	}
	if st.Searches != 1 || st.ListingsFetched != 1 || st.ListingsMatched != 1 {
		t.Errorf("stats: searches=%d fetched=%d matched=%d, want 1/1/1",
			st.Searches, st.ListingsFetched, st.ListingsMatched)
	}

	// A second chunk of the SAME cycle accumulates stats but keeps the schedule.
	w = doRequest(t, srv, "POST", "/api/v1/ext/ingest", map[string]any{
		"listings": []map[string]any{extIngestListing("ext-cycle-2")},
		"cycle":    cycle,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ingest chunk 2: %d %s", w.Code, w.Body.String())
	}
	st = getSchedulerStatus(t, srv)
	if st.ListingsFetched != 2 || st.ListingsMatched != 2 {
		t.Errorf("chunked stats: fetched=%d matched=%d, want 2/2", st.ListingsFetched, st.ListingsMatched)
	}
	if st.NextCycleAt == nil || *st.NextCycleAt != next.Format(time.RFC3339) {
		t.Errorf("next_cycle_at after chunk 2 = %v, want %s", st.NextCycleAt, next.Format(time.RFC3339))
	}
}

// A cycle that matched nothing still reports its schedule: the push carries
// only the cycle object and must be accepted, not short-circuited.
func TestIngestCycleOnlyPushRecordsSchedule(t *testing.T) {
	srv, _ := setupTestServer(t)

	next := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Second)
	w := doRequest(t, srv, "POST", "/api/v1/ext/ingest", map[string]any{
		"listings": []map[string]any{},
		"cycle": map[string]any{
			"started_at":   time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
			"next_run_at":  next.Format(time.RFC3339),
			"interval_sec": 900,
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("cycle-only ingest: %d %s", w.Code, w.Body.String())
	}

	st := getSchedulerStatus(t, srv)
	if st.NextCycleAt == nil || *st.NextCycleAt != next.Format(time.RFC3339) {
		t.Errorf("next_cycle_at = %v, want %s", st.NextCycleAt, next.Format(time.RFC3339))
	}
	if st.ListingsFetched != 0 || st.ListingsMatched != 0 {
		t.Errorf("stats should be zero: %+v", st)
	}
}

// While a scan is overdue by less than one interval (the alarm just fired and
// the cycle is still running), the report is still the best answer — the UI
// renders the overdue state as "scanning now".
func TestSchedulerStatusServesRecentlyOverdueReport(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	next := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	if err := store.UpsertExtScanStatus(ctx, storage.ExtScanStatus{
		ChatID:      999,
		StartedAt:   next.Add(-15 * time.Minute),
		NextRunAt:   next,
		IntervalSec: 900,
	}); err != nil {
		t.Fatal(err)
	}

	st := getSchedulerStatus(t, srv)
	if st.NextCycleAt == nil || *st.NextCycleAt != next.Format(time.RFC3339) {
		t.Errorf("next_cycle_at = %v, want %s (recently overdue must still serve)", st.NextCycleAt, next.Format(time.RFC3339))
	}
}

// Once the report is stale (more than one interval past its promised next
// run — Chrome closed, extension gone), fall back to the legacy estimate
// instead of freezing the countdown on a dead alarm.
func TestSchedulerStatusFallsBackWhenReportStale(t *testing.T) {
	srv, store := setupTestServer(t)
	ctx := context.Background()

	if err := store.UpsertExtScanStatus(ctx, storage.ExtScanStatus{
		ChatID:      999,
		StartedAt:   time.Now().UTC().Add(-40 * time.Minute),
		NextRunAt:   time.Now().UTC().Add(-20 * time.Minute),
		IntervalSec: 900,
	}); err != nil {
		t.Fatal(err)
	}

	// The test server wires no cycle log and no polling interval, so the
	// fallback is an empty status: next_cycle_at null is the fallback marker.
	st := getSchedulerStatus(t, srv)
	if st.NextCycleAt != nil {
		t.Errorf("stale report should not be served, got next_cycle_at=%s", *st.NextCycleAt)
	}
}

// A malformed next_run_at must not fail the ingest that carried it — and must
// not be served either.
func TestIngestBadCycleReportIsDropped(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/ext/ingest", map[string]any{
		"listings": []map[string]any{},
		"cycle": map[string]any{
			"started_at":   "not-a-time",
			"next_run_at":  "also-not-a-time",
			"interval_sec": 900,
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ingest with bad cycle should still 200: %d %s", w.Code, w.Body.String())
	}

	st := getSchedulerStatus(t, srv)
	if st.NextCycleAt != nil {
		t.Errorf("bad report should be dropped, got next_cycle_at=%s", *st.NextCycleAt)
	}
}

// A skewed client clock promising a next run beyond the longest allowed
// cadence would freeze the countdown on a moment that never arrives (the
// staleness check cannot catch a future timestamp) — such reports are dropped.
func TestIngestFarFutureCycleReportIsDropped(t *testing.T) {
	srv, _ := setupTestServer(t)

	w := doRequest(t, srv, "POST", "/api/v1/ext/ingest", map[string]any{
		"listings": []map[string]any{},
		"cycle": map[string]any{
			"started_at":   time.Now().UTC().Format(time.RFC3339),
			"next_run_at":  time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339),
			"interval_sec": 900,
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("ingest with far-future cycle should still 200: %d %s", w.Code, w.Body.String())
	}

	st := getSchedulerStatus(t, srv)
	if st.NextCycleAt != nil {
		t.Errorf("far-future report should be dropped, got next_cycle_at=%s", *st.NextCycleAt)
	}
}

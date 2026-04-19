package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatus_OK(t *testing.T) {
	s := New()
	s.RecordSuccess()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	if resp["cycles"].(float64) != 1 {
		t.Errorf("cycles = %v, want 1", resp["cycles"])
	}
}

func TestStatus_RecordError(t *testing.T) {
	s := New()
	s.RecordError()
	s.RecordError()
	s.RecordSuccess()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler()(w, req)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["errors"].(float64) != 2 {
		t.Errorf("errors = %v, want 2", resp["errors"])
	}
	if resp["cycles"].(float64) != 3 {
		t.Errorf("cycles = %v, want 3", resp["cycles"])
	}
}

func TestStatus_RecordListingsFound(t *testing.T) {
	s := New()
	s.RecordListingsFound(5)
	s.RecordListingsFound(3)

	snap := s.Snapshot()
	got := snap["listings_found"].(int64)
	if got != 8 {
		t.Errorf("listings_found = %d, want 8", got)
	}
}

func TestStatus_RecordNotificationSent(t *testing.T) {
	s := New()
	s.RecordNotificationSent()
	s.RecordNotificationSent()
	s.RecordNotificationSent()

	snap := s.Snapshot()
	got := snap["notifications_sent"].(int64)
	if got != 3 {
		t.Errorf("notifications_sent = %d, want 3", got)
	}
}

func TestStatus_ListingsAndNotificationsInHandler(t *testing.T) {
	s := New()
	s.RecordSuccess()
	s.RecordListingsFound(10)
	s.RecordNotificationSent()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["listings_found"].(float64) != 10 {
		t.Errorf("listings_found = %v, want 10", resp["listings_found"])
	}
	if resp["notifications_sent"].(float64) != 1 {
		t.Errorf("notifications_sent = %v, want 1", resp["notifications_sent"])
	}
}

// stubUserCounter implements UserCounter for testing.
type stubUserCounter struct{ n int64 }

func (s *stubUserCounter) CountUsers(_ context.Context) (int64, error) { return s.n, nil }

// stubSearchCounter implements SearchCounter for testing.
type stubSearchCounter struct{ n int64 }

func (s *stubSearchCounter) CountAllSearches(_ context.Context) (int64, error) { return s.n, nil }

func TestStatus_WithStoreCounters(t *testing.T) {
	s := New()
	s.SetUserCounter(&stubUserCounter{n: 42})
	s.SetSearchCounter(&stubSearchCounter{n: 7})
	s.RecordSuccess()

	snap := s.Snapshot()

	if snap["active_users"].(int64) != 42 {
		t.Errorf("active_users = %v, want 42", snap["active_users"])
	}
	if snap["active_searches"].(int64) != 7 {
		t.Errorf("active_searches = %v, want 7", snap["active_searches"])
	}
}

func TestStatus_WithoutStoreCounters(t *testing.T) {
	s := New()
	s.RecordSuccess()

	snap := s.Snapshot()

	if _, ok := snap["active_users"]; ok {
		t.Error("active_users should not be present without UserCounter")
	}
	if _, ok := snap["active_searches"]; ok {
		t.Error("active_searches should not be present without SearchCounter")
	}
}

func TestSnapshot_ReturnsSameDataAsHandler(t *testing.T) {
	s := New()
	s.RecordSuccess()
	s.RecordListingsFound(3)
	s.RecordNotificationSent()

	snap := s.Snapshot()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler()(w, req)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// Compare key fields (handler serialises via JSON so values become float64).
	if snap["status"] != resp["status"] {
		t.Errorf("status mismatch: snapshot=%v handler=%v", snap["status"], resp["status"])
	}
	if float64(snap["listings_found"].(int64)) != resp["listings_found"].(float64) {
		t.Errorf("listings_found mismatch")
	}
}

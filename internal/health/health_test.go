package health

import (
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
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["errors"].(float64) != 2 {
		t.Errorf("errors = %v, want 2", resp["errors"])
	}
	if resp["cycles"].(float64) != 3 {
		t.Errorf("cycles = %v, want 3", resp["cycles"])
	}
}

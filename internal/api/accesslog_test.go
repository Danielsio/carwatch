package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusRecorder_WriteHeaderFirstWins(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}
	sr.WriteHeader(http.StatusTeapot)
	sr.WriteHeader(http.StatusOK)
	if sr.status != http.StatusTeapot {
		t.Fatalf("recorded status = %d, want %d", sr.status, http.StatusTeapot)
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("underlying code = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

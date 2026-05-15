package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type flushSpy struct {
	http.ResponseWriter
	flushed bool
}

func (f *flushSpy) Flush() { f.flushed = true }

func TestStatusRecorder_ImplementsFlusher(t *testing.T) {
	spy := &flushSpy{ResponseWriter: httptest.NewRecorder()}
	sr := &statusRecorder{ResponseWriter: spy, status: http.StatusOK}
	var _ http.Flusher = sr // compile-time check
	sr.Flush()
	if !spy.flushed {
		t.Fatal("expected Flush to be delegated to underlying writer")
	}
}

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

package fetcher

import (
	"log/slog"
	"testing"
	"time"
)

func TestWithCBLogger(t *testing.T) {
	inner := &mockFetcherCB{}
	l := slog.Default()
	cb := NewCircuitBreaker(inner, 3, 5*time.Second, WithCBLogger(l))
	if cb.logger != l {
		t.Error("expected custom logger to be set")
	}
}

func TestWithCBLogger_Nil(t *testing.T) {
	inner := &mockFetcherCB{}
	cb := NewCircuitBreaker(inner, 3, 5*time.Second, WithCBLogger(nil))
	if cb.logger == nil {
		t.Error("nil logger should not replace default")
	}
}

package scheduler

import (
	"context"
	"errors"
	"testing"
)

// TestFlushPendingPrices covers the staged-price flush rules (F4): a price is
// durably recorded only when its listing's outcome settled — persisted in at
// least one search, or never part of a persist at all (price-drop-only
// tokens, whose history rows already exist).
func TestFlushPendingPrices(t *testing.T) {
	tests := []struct {
		name          string
		pending       map[string]int
		persisted     map[string]bool
		persistFailed map[string]bool
		wantRecorded  map[string]int
	}{
		{
			name:         "persisted token is flushed",
			pending:      map[string]int{"tok-ok": 90000},
			persisted:    map[string]bool{"tok-ok": true},
			wantRecorded: map[string]int{"tok-ok": 90000},
		},
		{
			name:          "failed-only token is skipped",
			pending:       map[string]int{"tok-fail": 90000},
			persistFailed: map[string]bool{"tok-fail": true},
			wantRecorded:  map[string]int{},
		},
		{
			name:          "token persisted by one search and failed by another is flushed",
			pending:       map[string]int{"tok-mixed": 80000},
			persisted:     map[string]bool{"tok-mixed": true},
			persistFailed: map[string]bool{"tok-mixed": true},
			wantRecorded:  map[string]int{"tok-mixed": 80000},
		},
		{
			name:         "drop-only token (no persist involvement) is flushed",
			pending:      map[string]int{"tok-drop": 70000},
			wantRecorded: map[string]int{"tok-drop": 70000},
		},
		{
			name:         "empty pending is a no-op",
			pending:      map[string]int{},
			wantRecorded: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := newMockPriceTracker()
			s := &Scheduler{
				logger: testLogger(),
				stores: Stores{Prices: pt},
			}
			s.flushPendingPrices(context.Background(), tt.pending, tt.persisted, tt.persistFailed)

			pt.mu.Lock()
			defer pt.mu.Unlock()
			if len(pt.prices) != len(tt.wantRecorded) {
				t.Fatalf("recorded %d prices, want %d: %v", len(pt.prices), len(tt.wantRecorded), pt.prices)
			}
			for token, price := range tt.wantRecorded {
				if got := pt.prices[token]; got != price {
					t.Errorf("price[%s] = %d, want %d", token, got, price)
				}
			}
		})
	}
}

// TestFlushPendingPrices_RecordErrorContinues verifies that a failing record
// neither panics nor stops the flush (a missed write self-corrects next cycle
// because PeekPrice still sees the old value).
func TestFlushPendingPrices_RecordErrorContinues(t *testing.T) {
	pt := newErrPriceTracker()
	pt.err = errors.New("db down")
	s := &Scheduler{logger: testLogger(), stores: Stores{Prices: pt}}

	s.flushPendingPrices(context.Background(),
		map[string]int{"a": 1, "b": 2}, nil, nil)
	// Nothing recorded, no panic.
	if len(pt.prices) != 0 {
		t.Fatalf("expected no recorded prices on error, got %v", pt.prices)
	}
}

// TestFlushPendingPrices_NilStore verifies the nil-prices no-op path.
func TestFlushPendingPrices_NilStore(t *testing.T) {
	s := &Scheduler{logger: testLogger()}
	s.flushPendingPrices(context.Background(), map[string]int{"a": 1}, nil, nil)
}

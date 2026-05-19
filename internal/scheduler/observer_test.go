package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNopObserver_DoesNotPanic(t *testing.T) {
	var o nopObserver
	o.RecordSuccess()
	o.RecordError()
	o.RecordListingsFound(10)
	o.RecordNotificationSent()
	o.RecordFetch("yad2", time.Second, nil)
}

type countingObserver struct {
	successes     atomic.Int64
	errors        atomic.Int64
	listingsFound atomic.Int64
	notifications atomic.Int64
	fetches       atomic.Int64
}

func (o *countingObserver) RecordSuccess()                                 { o.successes.Add(1) }
func (o *countingObserver) RecordError()                                   { o.errors.Add(1) }
func (o *countingObserver) RecordListingsFound(n int)                      { o.listingsFound.Add(int64(n)) }
func (o *countingObserver) RecordNotificationSent()                        { o.notifications.Add(1) }
func (o *countingObserver) RecordFetch(_ string, _ time.Duration, _ error) { o.fetches.Add(1) }

func TestCycleObserver_Interface(t *testing.T) {
	var obs CycleObserver = &countingObserver{}
	obs.RecordSuccess()
	obs.RecordError()
	obs.RecordListingsFound(5)
	obs.RecordNotificationSent()

	co := obs.(*countingObserver)
	if v := co.successes.Load(); v != 1 {
		t.Errorf("successes = %d, want 1", v)
	}
	if v := co.errors.Load(); v != 1 {
		t.Errorf("errors = %d, want 1", v)
	}
	if v := co.listingsFound.Load(); v != 5 {
		t.Errorf("listingsFound = %d, want 5", v)
	}
	if v := co.notifications.Load(); v != 1 {
		t.Errorf("notifications = %d, want 1", v)
	}
}

func TestNewScheduler_NilObserver_UsesNop(t *testing.T) {
	cfg := testConfig()
	s, err := NewWithOptions(cfg, nil, nil, nil, testLogger(), Options{Observer: nil})
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	_, isNop := s.observer.(nopObserver)
	if !isNop {
		t.Errorf("expected nopObserver when nil, got %T", s.observer)
	}
}

package scheduler

import (
	"testing"
)

func TestNopObserver_DoesNotPanic(t *testing.T) {
	var o nopObserver
	o.RecordSuccess()
	o.RecordError()
	o.RecordListingsFound(10)
	o.RecordNotificationSent()
}

type countingObserver struct {
	successes     int
	errors        int
	listingsFound int
	notifications int
}

func (o *countingObserver) RecordSuccess()          { o.successes++ }
func (o *countingObserver) RecordError()            { o.errors++ }
func (o *countingObserver) RecordListingsFound(n int) { o.listingsFound += n }
func (o *countingObserver) RecordNotificationSent() { o.notifications++ }

func TestCycleObserver_Interface(t *testing.T) {
	var obs CycleObserver = &countingObserver{}
	obs.RecordSuccess()
	obs.RecordError()
	obs.RecordListingsFound(5)
	obs.RecordNotificationSent()

	co := obs.(*countingObserver)
	if co.successes != 1 {
		t.Errorf("successes = %d, want 1", co.successes)
	}
	if co.errors != 1 {
		t.Errorf("errors = %d, want 1", co.errors)
	}
	if co.listingsFound != 5 {
		t.Errorf("listingsFound = %d, want 5", co.listingsFound)
	}
	if co.notifications != 1 {
		t.Errorf("notifications = %d, want 1", co.notifications)
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

package telemetry

import (
	"context"
	"testing"
)

func TestInitMetrics(t *testing.T) {
	if err := InitMetrics(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All metric instruments should be non-nil after init.
	if ScrapesDuration == nil {
		t.Error("ScrapesDuration is nil")
	}
	if ListingsFetched == nil {
		t.Error("ListingsFetched is nil")
	}
	if ListingsMatched == nil {
		t.Error("ListingsMatched is nil")
	}
	if NotificationsSent == nil {
		t.Error("NotificationsSent is nil")
	}
	if SchedulerCycles == nil {
		t.Error("SchedulerCycles is nil")
	}
	if ActiveSearches == nil {
		t.Error("ActiveSearches is nil")
	}
	if ActiveUsers == nil {
		t.Error("ActiveUsers is nil")
	}
}

func TestInitMetrics_Record(t *testing.T) {
	if err := InitMetrics(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	// Recording should not panic.
	ScrapesDuration.Record(ctx, 1.5)
	ListingsFetched.Add(ctx, 10)
	ListingsMatched.Add(ctx, 3)
	NotificationsSent.Add(ctx, 1)
	SchedulerCycles.Add(ctx, 1)
	ActiveSearches.Record(ctx, 5)
	ActiveUsers.Record(ctx, 2)
}

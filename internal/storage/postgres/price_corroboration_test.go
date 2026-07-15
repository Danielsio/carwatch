package postgres

import (
	"context"
	"testing"
)

func TestImplausiblePriceSwing(t *testing.T) {
	cases := []struct {
		name       string
		prev, next int
		want       bool
	}{
		{"normal drop", 80000, 74000, false},
		{"normal rise", 74000, 78000, false},
		{"exactly 3x is allowed", 20000, 60000, false},
		{"more than 3x is rejected", 20000, 61000, true},
		{"collapse to a third is allowed", 60000, 20000, false},
		{"collapse below a third is rejected", 60000, 19000, true},
		{"fabricated near-zero drop", 80000, 1, true},
		{"absurd spike", 80000, 999999999, true},
		{"zero next is always implausible", 80000, 0, true},
		{"negative next", 80000, -5, true},
		{"no prior price, positive is fine", 0, 50000, false},
		{"no prior price, non-positive is not", 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := implausiblePriceSwing(c.prev, c.next); got != c.want {
				t.Errorf("implausiblePriceSwing(%d, %d) = %v, want %v", c.prev, c.next, got, c.want)
			}
		})
	}
}

// The cross-tenant poisoning scenario: a second user watching the same car
// pushes a fabricated steep drop. It must neither enter the shared history nor
// be reported as a change (which is what would fire a false "price dropped"
// alert to the first user).
func TestRecordPrice_RejectsAFabricatedSteepDrop(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	const token = "shared-car"

	// User A's genuine price is observed first.
	if _, _, err := store.RecordPrice(ctx, token, 80000); err != nil {
		t.Fatal(err)
	}

	// User B pushes a fabricated collapse.
	old, changed, err := store.RecordPrice(ctx, token, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("a fabricated steep drop was reported as a change — it would fire a false alert")
	}
	if old != 80000 {
		t.Fatalf("expected the real price preserved, got %d", old)
	}

	// The history still shows only the genuine price.
	hist, err := store.GetPriceHistory(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].Price != 80000 {
		t.Fatalf("fabricated price poisoned the history: %+v", hist)
	}
}

// A genuine, plausible price drop must still be recorded and reported — that is
// the product's whole point.
func TestRecordPrice_RecordsAPlausibleDrop(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	const token = "dropping-car"

	if _, _, err := store.RecordPrice(ctx, token, 80000); err != nil {
		t.Fatal(err)
	}
	old, changed, err := store.RecordPrice(ctx, token, 73000) // a real ~9% cut
	if err != nil {
		t.Fatal(err)
	}
	if !changed || old != 80000 {
		t.Fatalf("a genuine price drop was not recorded: changed=%v old=%d", changed, old)
	}
	hist, err := store.GetPriceHistory(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Fatalf("expected two price points, got %d", len(hist))
	}
}

// A non-positive first observation must not seed the history with junk.
func TestRecordPrice_IgnoresANonPositiveFirstObservation(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	const token = "priceless"

	if _, _, err := store.RecordPrice(ctx, token, 0); err != nil {
		t.Fatal(err)
	}
	hist, err := store.GetPriceHistory(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 0 {
		t.Fatalf("a zero price seeded the history: %+v", hist)
	}
}

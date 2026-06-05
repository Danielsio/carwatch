package postgres

import (
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/storage"
)

func TestCountSearchListingsForChatQuery_Contract(t *testing.T) {
	for _, frag := range []string{
		"FROM listing_history lh",
		"INNER JOIN searches s",
		"s.price_max <= 0 OR lh.price <= s.price_max",
		"s.year_min <= 0 OR lh.year >= s.year_min",
		"max_km <= 0 OR lh.km <= s.max_km OR lh.km = 0",
		"s.seller_filter",
		"WHEN 'dealer'",
		"lh.is_commercial",
		"lh.removed_at IS NULL",
		"GROUP BY lh.search_id",
	} {
		if !strings.Contains(countSearchListingsForChatSQL, frag) {
			t.Errorf("countSearchListingsForChatSQL missing %q", frag)
		}
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users", `"users"`},
		{"listing_history", `"listing_history"`},
		{`evil"table`, `"evil""table"`},
		{`a""b`, `"a""""b"`},
		{"", `""`},
	}

	for _, tt := range tests {
		got := quoteIdent(tt.input)
		if got != tt.want {
			t.Errorf("quoteIdent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildFilterClauses_Empty(t *testing.T) {
	clause, args, nextParam := buildFilterClauses(storage.ListingFilter{}, 1)
	if !strings.Contains(clause, "removed_at IS NULL") {
		t.Errorf("expected removed_at IS NULL clause, got %q", clause)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
	if nextParam != 1 {
		t.Errorf("expected nextParam=1, got %d", nextParam)
	}
}

func TestBuildFilterClauses_IncludeRemoved(t *testing.T) {
	clause, args, nextParam := buildFilterClauses(storage.ListingFilter{IncludeRemoved: true}, 1)
	if clause != "" {
		t.Errorf("expected empty clause with IncludeRemoved, got %q", clause)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
	if nextParam != 1 {
		t.Errorf("expected nextParam=1, got %d", nextParam)
	}
}

func TestBuildFilterClauses_AllFields(t *testing.T) {
	f := storage.ListingFilter{
		PriceMax: 100000,
		YearMin:  2018,
		YearMax:  2023,
		MaxKm:    150000,
		MaxHand:  2,
	}
	clause, args, nextParam := buildFilterClauses(f, 3)

	if clause == "" {
		t.Fatal("expected non-empty clause")
	}

	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d: %v", len(args), args)
	}

	if args[0] != 100000 {
		t.Errorf("args[0] = %v, want 100000", args[0])
	}
	if args[1] != 2018 {
		t.Errorf("args[1] = %v, want 2018", args[1])
	}
	if args[2] != 2023 {
		t.Errorf("args[2] = %v, want 2023", args[2])
	}
	if args[3] != 150000 {
		t.Errorf("args[3] = %v, want 150000", args[3])
	}
	if args[4] != 2 {
		t.Errorf("args[4] = %v, want 2", args[4])
	}

	if nextParam != 8 {
		t.Errorf("nextParam = %d, want 8 (started at 3, used 5)", nextParam)
	}
}

func TestBuildFilterClauses_PartialFields(t *testing.T) {
	f := storage.ListingFilter{
		PriceMax: 70000,
		MaxKm:    120000,
	}
	clause, args, nextParam := buildFilterClauses(f, 5)

	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != 70000 {
		t.Errorf("args[0] = %v, want 70000", args[0])
	}
	if args[1] != 120000 {
		t.Errorf("args[1] = %v, want 120000", args[1])
	}
	if nextParam != 7 {
		t.Errorf("nextParam = %d, want 7", nextParam)
	}
	if clause == "" {
		t.Error("expected non-empty clause")
	}
}

func TestBuildFilterClauses_MaxKmIncludesZeroKm(t *testing.T) {
	f := storage.ListingFilter{MaxKm: 50000}
	clause, _, _ := buildFilterClauses(f, 1)

	if clause == "" {
		t.Fatal("expected clause")
	}
	// km=0 listings should be included (OR km = 0), not excluded
	if !strings.Contains(clause, "km = 0") {
		t.Errorf("MaxKm clause should include 'km = 0' to pass unknown km, got: %s", clause)
	}
	if !strings.Contains(clause, "km <= $1") {
		t.Errorf("MaxKm clause should include 'km <= $1', got: %s", clause)
	}
}

func TestBuildFilterClauses_Commercial(t *testing.T) {
	tVal := true
	f := storage.ListingFilter{Commercial: &tVal}
	clause, args, next := buildFilterClauses(f, 2)
	if !strings.Contains(clause, "is_commercial = $2") {
		t.Fatalf("expected is_commercial placeholder, got %q", clause)
	}
	if len(args) != 1 || next != 3 {
		t.Fatalf("args=%v next=%d", args, next)
	}

	fVal := false
	f2 := storage.ListingFilter{Commercial: &fVal}
	clause2, args2, next2 := buildFilterClauses(f2, 1)
	if !strings.Contains(clause2, "is_commercial = $1") {
		t.Fatalf("expected private clause, got %q", clause2)
	}
	if len(args2) != 1 || next2 != 2 {
		t.Fatalf("args2=%v next2=%d", args2, next2)
	}
}

func TestGenerateShareToken(t *testing.T) {
	token, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 32 {
		t.Errorf("expected 32-char hex token, got len=%d: %q", len(token), token)
	}

	token2, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error on second token: %v", err)
	}
	if token == token2 {
		t.Error("two consecutive tokens should differ")
	}
}

func TestPurgeableAllowlist(t *testing.T) {
	allowed := []string{
		"listing_history",
		"price_history",
		"seen_listings",
		"listing_user_seen",
		"saved_listings",
		"hidden_listings",
		"pending_digest",
	}
	for _, table := range allowed {
		if !purgeable[table] {
			t.Errorf("%q should be purgeable", table)
		}
	}

	forbidden := []string{"users", "searches", "link_tokens", "whatever"}
	for _, table := range forbidden {
		if purgeable[table] {
			t.Errorf("%q should NOT be purgeable", table)
		}
	}
}

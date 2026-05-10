package storage

import "testing"

func TestNormalizeSellerFilter(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", SellerFilterAny},
		{"any", SellerFilterAny},
		{"ANY", SellerFilterAny},
		{"private", SellerFilterPrivate},
		{" commercial ", SellerFilterCommercial},
		{"dealer", SellerFilterCommercial},
		{"dealership", SellerFilterCommercial},
		{"bogus", SellerFilterAny},
	}
	for _, tt := range tests {
		if got := NormalizeSellerFilter(tt.in); got != tt.want {
			t.Errorf("NormalizeSellerFilter(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRawListingMatchesSellerFilter(t *testing.T) {
	priv := false
	dealer := true

	tests := []struct {
		name       string
		commerce   *bool
		filter     string
		wantMatch  bool
	}{
		{"any nil", nil, SellerFilterAny, true},
		{"any private", &priv, SellerFilterAny, true},
		{"private matches", &priv, SellerFilterPrivate, true},
		{"private dealer no", &dealer, SellerFilterPrivate, false},
		{"private unknown no", nil, SellerFilterPrivate, false},
		{"commercial matches", &dealer, SellerFilterCommercial, true},
		{"commercial private no", &priv, SellerFilterCommercial, false},
		{"commercial unknown no", nil, SellerFilterCommercial, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RawListingMatchesSellerFilter(tt.commerce, tt.filter); got != tt.wantMatch {
				t.Fatalf("RawListingMatchesSellerFilter(%v, %q) = %v, want %v", tt.commerce, tt.filter, got, tt.wantMatch)
			}
		})
	}
}

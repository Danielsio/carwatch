package storage

import "strings"

// Seller filter values (stored on Search.seller_filter).
const (
	SellerFilterAny        = "any"
	SellerFilterPrivate    = "private"
	SellerFilterCommercial = "commercial"
)

// NormalizeSellerFilter returns any | private | commercial; invalid input becomes "any".
func NormalizeSellerFilter(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case SellerFilterPrivate:
		return SellerFilterPrivate
	case SellerFilterCommercial, "dealer", "dealership":
		return SellerFilterCommercial
	default:
		return SellerFilterAny
	}
}

// RawListingMatchesSellerFilter returns whether a scraped listing should be processed
// for searches with the given seller_filter (use NormalizeSellerFilter first), based on
// Commercial: nil = unknown (does not match private or commercial-only filters).
func RawListingMatchesSellerFilter(commercial *bool, sellerFilter string) bool {
	switch NormalizeSellerFilter(sellerFilter) {
	case SellerFilterPrivate:
		return commercial != nil && !*commercial
	case SellerFilterCommercial:
		return commercial != nil && *commercial
	default:
		return true
	}
}

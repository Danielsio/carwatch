package scoring

import "github.com/dsionov/carwatch/internal/model"

// DetectSuspicious returns a list of reason codes if the listing looks suspicious.
// An empty/nil slice means no suspicion was detected.
func DetectSuspicious(listing model.RawListing, medianPrice int) []string {
	var reasons []string

	// Price dramatically below market (< 50% of median).
	if medianPrice > 0 && listing.Price > 0 && listing.Price < medianPrice/2 {
		reasons = append(reasons, "price_below_market")
	}

	// No photos with very low price (< 70% of median).
	if listing.ImageURL == "" && medianPrice > 0 && listing.Price > 0 && listing.Price < medianPrice*70/100 {
		reasons = append(reasons, "no_photo_low_price")
	}

	return reasons
}

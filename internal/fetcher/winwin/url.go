package winwin

import (
	"net/url"
	"strconv"

	"github.com/dsionov/carwatch/internal/model"
)

const defaultBaseURL = "https://www.winwin.co.il/vehicles/cars"

// buildURL constructs a WinWin search URL from the given parameters.
// TODO: Reverse-engineer the actual WinWin URL structure and query parameters.
// The parameters below are placeholders based on common patterns.
func buildURL(base string, params model.SourceParams) string {
	u, _ := url.Parse(base)
	v := url.Values{}

	if params.Manufacturer > 0 {
		v.Set("manufacturer", strconv.Itoa(params.Manufacturer))
	}
	if params.Model > 0 {
		v.Set("model", strconv.Itoa(params.Model))
	}
	if params.YearMin > 0 {
		v.Set("yearFrom", strconv.Itoa(params.YearMin))
	}
	if params.YearMax > 0 {
		v.Set("yearTo", strconv.Itoa(params.YearMax))
	}
	if params.PriceMin > 0 {
		v.Set("priceFrom", strconv.Itoa(params.PriceMin))
	}
	if params.PriceMax > 0 {
		v.Set("priceTo", strconv.Itoa(params.PriceMax))
	}
	if params.Page > 0 {
		v.Set("page", strconv.Itoa(params.Page))
	}

	u.RawQuery = v.Encode()
	return u.String()
}

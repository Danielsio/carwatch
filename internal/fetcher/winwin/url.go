package winwin

import (
	"net/url"
	"strconv"

	"github.com/dsionov/carwatch/internal/model"
)

const defaultBaseURL = "https://www.winwin.co.il/vehicles/cars-for-sale"

func buildURL(base string, params model.SourceParams) string {
	u, err := url.Parse(base)
	if err != nil {
		u = &url.URL{Path: base}
	}
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
	if params.MaxKm > 0 {
		v.Set("km", strconv.Itoa(params.MaxKm))
	}
	if params.MaxHand > 0 {
		v.Set("hand", strconv.Itoa(params.MaxHand))
	}
	if params.EngineMinCC > 0 {
		v.Set("engineVolume", strconv.Itoa(params.EngineMinCC))
	}
	if params.Page > 0 {
		v.Set("page", strconv.Itoa(params.Page))
	}
	u.RawQuery = v.Encode()
	return u.String()
}

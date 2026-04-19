package winwin

import (
	"io"

	"github.com/dsionov/carwatch/internal/model"
)

// ParseListingsPage parses WinWin HTML into raw listings.
// TODO: Implement actual HTML parsing once the WinWin page structure is
// reverse-engineered. The parser should extract listing tokens, prices,
// years, mileage, etc. from the HTML response body.
func ParseListingsPage(_ io.Reader) ([]model.RawListing, error) {
	return nil, nil
}

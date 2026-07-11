// Package timeutil holds the timestamp parsing shared by the two paths that
// ingest Yad2 listings — the scraper (internal/fetcher/yad2) and the extension
// ingest endpoint (internal/api). Both read the same feed, so they must agree
// on how its dates are interpreted; keeping one implementation here stops them
// from drifting apart.
package timeutil

import (
	"strings"
	"time"
)

// IsraelTZ interprets the Yad2 feed's zone-less local timestamps. Falls back to
// UTC if the zoneinfo database is unavailable.
var IsraelTZ = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jerusalem")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// ParseFlexTime parses the timestamp formats Yad2 has been observed to emit.
// Yad2's feed dates are zone-less local time (e.g. "2025-02-09T10:31:37"), but
// it has historically drifted between "Z"/offset-suffixed and fractional-second
// variants, so we accept all of them rather than silently dropping the date.
//
// Zone-less values MUST be read as Israel local time: parsing them as UTC puts
// posted_at ~2-3h early for every listing.
//
// Returns the zero time when s is empty or unparseable.
func ParseFlexTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// Zone-bearing formats ("...Z" or "...+02:00"), with or without fractional
	// seconds. RFC3339Nano covers the fractional case; RFC3339 the rest.
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	// Zone-less formats — interpret as Israel local time, matching the feed.
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, s, IsraelTZ); err == nil {
			return t
		}
	}
	return time.Time{}
}

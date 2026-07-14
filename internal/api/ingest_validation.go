package api

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The ingest endpoint is a trust boundary, not a private channel. It is
// reachable by any authenticated user — the extension is merely its expected
// client — and what it stores is later rendered as links and images in the web
// UI and embedded in Telegram messages. So an ingested listing is treated as
// what it is: attacker-controllable text and URLs.
//
// Values that fail validation are blanked rather than rejected: a single odd
// field must not cost the user the whole cycle's listings. The token, which is
// the listing's identity, is the one exception — a malformed token is dropped.

// Field length caps. Yad2's real values are far below these; the caps exist so
// a client cannot store megabytes of text per listing (the 1 MB body cap alone
// would still allow it across many listings) or push a payload that breaks
// rendering downstream.
const (
	maxDescriptionLen = 4000
	maxNameFieldLen   = 200
	maxURLLen         = 2048
)

// yad2TokenRe matches the shape of a Yad2 ad token: an opaque alphanumeric id.
// Anything else is not a listing identity we can act on — and since the token
// is what dedup and removal (MarkListingsRemoved) key on, junk here is worse
// than useless.
//
// The character class does the security work (no path separators, quotes,
// angle brackets, whitespace or non-ASCII); the length bound is deliberately
// generous. Yad2 has never published its token format and it has changed
// before, so a tight length would risk silently dropping every real listing the
// day they lengthen it — the exact failure this validation exists to prevent.
var yad2TokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// validIngestToken reports whether a token can identify a listing.
func validIngestToken(token string) bool {
	return yad2TokenRe.MatchString(token)
}

// allowedListingHosts are the hosts a listing's page may live on. The page_link
// is presented to the user as *the* link to the ad — in the UI and inside
// Telegram alerts — so allowing an arbitrary host would let one account deliver
// a phishing link that arrives wearing CarWatch's own branding.
var allowedListingHosts = []string{
	"yad2.co.il",
}

// allowedImageHosts are the hosts a listing image may be loaded from. A foreign
// image URL is both an exfiltration beacon (it reports every viewer's IP and
// user-agent to whoever controls it) and a way to render arbitrary content in
// the UI.
var allowedImageHosts = []string{
	"yad2.co.il",
	"yit.co.il",
}

// hostAllowed reports whether host is one of allowed, or a subdomain of one.
func hostAllowed(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, a := range allowed {
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// sanitizeURL returns raw when it is an https URL on an allowed host, and ""
// otherwise. Empty in, empty out — a listing with no image is normal.
func sanitizeURL(raw string, allowed []string) string {
	if raw == "" || len(raw) > maxURLLen {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return ""
	}
	if !hostAllowed(u.Hostname(), allowed) {
		return ""
	}
	return raw
}

// truncateRunes caps s at max runes, cutting on a rune boundary so the result
// is always valid UTF-8 (these fields are Hebrew; a byte-wise cut would corrupt
// the final character).
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	count := 0
	for i := range s {
		count++
		if count > max {
			return s[:i]
		}
	}
	return s
}

// sanitizeText trims a free-text field and caps its length. It also strips
// control characters, which have no place in a listing field and can garble
// logs and Telegram messages.
func sanitizeText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r // legitimate inside a description
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return truncateRunes(strings.TrimSpace(s), max)
}

// sanitizeIngestListing normalizes one submitted listing in place, returning
// false when it cannot be stored at all (no usable token).
func sanitizeIngestListing(l *ingestListing) bool {
	l.Token = strings.TrimSpace(l.Token)
	if !validIngestToken(l.Token) {
		return false
	}

	l.PageLink = sanitizeURL(l.PageLink, allowedListingHosts)
	l.ImageURL = sanitizeURL(l.ImageURL, allowedImageHosts)

	l.Manufacturer = sanitizeText(l.Manufacturer, maxNameFieldLen)
	l.Model = sanitizeText(l.Model, maxNameFieldLen)
	l.SubModel = sanitizeText(l.SubModel, maxNameFieldLen)
	l.BodyType = sanitizeText(l.BodyType, maxNameFieldLen)
	l.City = sanitizeText(l.City, maxNameFieldLen)
	l.Area = sanitizeText(l.Area, maxNameFieldLen)
	l.EngineType = sanitizeText(l.EngineType, maxNameFieldLen)
	l.GearBox = sanitizeText(l.GearBox, maxNameFieldLen)
	l.Description = sanitizeText(l.Description, maxDescriptionLen)

	return true
}

// sanitizeRemovedTokens keeps only well-formed tokens. Removal deletes rows, so
// a malformed token is never worth acting on.
func sanitizeRemovedTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if validIngestToken(t) {
			out = append(out, t)
		}
	}
	return out
}

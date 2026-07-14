package api

import (
	"strings"
	"testing"
)

func TestValidIngestToken(t *testing.T) {
	// Yad2's token format is opaque and has changed before, so the length bound
	// is generous on purpose — the character class is what does the security
	// work. A tight bound would silently drop every real listing the day Yad2
	// lengthens its ids.
	valid := []string{"a", "abc", "abc123", "A1b2C3d4", "tok_en-1", strings.Repeat("a", 64)}
	for _, tok := range valid {
		if !validIngestToken(tok) {
			t.Errorf("token %q should be accepted", tok)
		}
	}

	invalid := []string{
		"",
		strings.Repeat("a", 65),     // absurdly long
		"../../etc/passwd",          // path traversal shape
		"tok en",                    // whitespace
		"tok'; DROP TABLE--",        // SQL-ish junk
		"<script>alert(1)</script>", // markup
		"טוקן",                      // non-ASCII
	}
	for _, tok := range invalid {
		if validIngestToken(tok) {
			t.Errorf("token %q should be rejected", tok)
		}
	}
}

// page_link is presented as *the* link to the ad — in the web UI and inside
// Telegram alerts. An arbitrary host there is a phishing link wearing CarWatch's
// branding, so anything off-Yad2 is dropped.
func TestSanitizeURL_ListingLink(t *testing.T) {
	keep := []string{
		"https://www.yad2.co.il/vehicles/item/abc123",
		"https://yad2.co.il/vehicles/item/abc123",
	}
	for _, u := range keep {
		if got := sanitizeURL(u, allowedListingHosts); got != u {
			t.Errorf("legitimate listing link %q was rejected (got %q)", u, got)
		}
	}

	drop := []string{
		"https://evil.example.com/phish",
		"https://yad2.co.il.evil.com/phish",  // suffix-confusion
		"https://notyad2.co.il/phish",        // must not match on a bare substring
		"http://www.yad2.co.il/vehicles/x",   // plaintext
		"javascript:alert(1)",                // scheme abuse
		"data:text/html;base64,PHNjcmlwdD4=", // inline payload
		"//evil.com/x",                       // scheme-relative
	}
	for _, u := range drop {
		if got := sanitizeURL(u, allowedListingHosts); got != "" {
			t.Errorf("hostile listing link %q was accepted as %q", u, got)
		}
	}
}

func TestSanitizeURL_Image(t *testing.T) {
	// The real Yad2 image host.
	const real = "https://img.yad2.co.il/Pic/202607/car.jpeg"
	if got := sanitizeURL(real, allowedImageHosts); got != real {
		t.Errorf("real Yad2 image host was rejected: %q", got)
	}
	// A foreign image is an exfiltration beacon: it reports every viewer's IP
	// and user-agent to whoever controls it.
	if got := sanitizeURL("https://tracker.example.com/beacon.png", allowedImageHosts); got != "" {
		t.Errorf("foreign image host was accepted as %q", got)
	}
	// Absurdly long URLs are rejected outright.
	if got := sanitizeURL("https://img.yad2.co.il/"+strings.Repeat("a", maxURLLen), allowedImageHosts); got != "" {
		t.Errorf("over-long URL was accepted as %q", got)
	}
}

func TestSanitizeText_CapsLengthOnRuneBoundaries(t *testing.T) {
	// These fields are Hebrew; a byte-wise cut would corrupt the final rune.
	long := strings.Repeat("א", maxDescriptionLen+500)
	got := sanitizeText(long, maxDescriptionLen)

	if !isValidUTF8(got) {
		t.Fatal("truncation produced invalid UTF-8")
	}
	if n := len([]rune(got)); n != maxDescriptionLen {
		t.Fatalf("expected %d runes, got %d", maxDescriptionLen, n)
	}
}

func TestSanitizeText_StripsControlCharactersButKeepsNewlines(t *testing.T) {
	got := sanitizeText("clean\x00text\x07here\nsecond line", maxDescriptionLen)
	if strings.ContainsAny(got, "\x00\x07") {
		t.Errorf("control characters survived: %q", got)
	}
	if !strings.Contains(got, "\nsecond line") {
		t.Errorf("newlines should survive inside a description: %q", got)
	}
}

func TestSanitizeIngestListing(t *testing.T) {
	t.Run("keeps a legitimate listing intact", func(t *testing.T) {
		l := ingestListing{
			Token:        "abc123",
			Manufacturer: "Mazda",
			Model:        "3",
			City:         "תל אביב",
			PageLink:     "https://www.yad2.co.il/vehicles/item/abc123",
			ImageURL:     "https://img.yad2.co.il/Pic/car.jpeg",
			Description:  "רכב שמור",
		}
		if !sanitizeIngestListing(&l) {
			t.Fatal("a legitimate listing was dropped")
		}
		if l.PageLink == "" || l.ImageURL == "" || l.City != "תל אביב" {
			t.Fatalf("legitimate fields were mangled: %+v", l)
		}
	})

	t.Run("blanks hostile URLs but keeps the listing", func(t *testing.T) {
		l := ingestListing{
			Token:    "abc123",
			PageLink: "https://evil.example.com/phish",
			ImageURL: "https://tracker.example.com/beacon.png",
			Model:    "3",
		}
		// One bad field must not cost the user the whole cycle's listings.
		if !sanitizeIngestListing(&l) {
			t.Fatal("listing was dropped when blanking the bad fields would do")
		}
		if l.PageLink != "" {
			t.Errorf("phishing page_link survived: %q", l.PageLink)
		}
		if l.ImageURL != "" {
			t.Errorf("beacon image_url survived: %q", l.ImageURL)
		}
		if l.Model != "3" {
			t.Errorf("good field was lost: %q", l.Model)
		}
	})

	t.Run("drops a listing with no usable token", func(t *testing.T) {
		// The token is the listing's identity — it is what dedup and removal key
		// on — so junk there is worse than useless.
		l := ingestListing{Token: "'; DROP TABLE listing_history--"}
		if sanitizeIngestListing(&l) {
			t.Fatal("listing with a malformed token was accepted")
		}
	})
}

func TestSanitizeRemovedTokens(t *testing.T) {
	// Removal deletes rows: only well-formed tokens are ever acted on.
	got := sanitizeRemovedTokens([]string{"abc123", "", "../../x", "def456", "<script>"})
	want := []string{"abc123", "def456"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

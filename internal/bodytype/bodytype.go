package bodytype

import (
	"regexp"
	"strings"
)

const (
	Sedan       = "sedan"
	Hatchback   = "hatchback"
	SUV         = "suv"
	Coupe       = "coupe"
	Wagon       = "wagon"
	Convertible = "convertible"
	Minivan     = "minivan"
	Pickup      = "pickup"
)

var all = [...]string{Sedan, Hatchback, SUV, Coupe, Wagon, Convertible, Minivan, Pickup}

func All() []string {
	out := make([]string, len(all))
	copy(out, all[:])
	return out
}

type matcher struct {
	bodyType string
	match    func(lower string) bool
}

func substr(s string) func(string) bool {
	return func(input string) bool { return strings.Contains(input, s) }
}

func word(w string) func(string) bool {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(w) + `\b`)
	return func(input string) bool { return re.MatchString(input) }
}

var matchers = []matcher{
	{Hatchback, substr("האצ'בק")},
	{Hatchback, substr("האצ׳בק")},
	{Hatchback, substr("ליפטבק")}, // Yad2 "ליפטבק" (liftback) — grouped with hatchback
	{Hatchback, substr("hatchback")},
	{Hatchback, substr("liftback")},
	{Hatchback, word("HB")},

	{Sedan, substr("סדאן")},
	{Sedan, substr("sedan")},

	{SUV, substr("פנאי-שטח")}, // Yad2 "פנאי-שטח" — the most common crossover/SUV category
	{SUV, substr("פנאי שטח")}, // spacing variant
	{SUV, substr("שטח קשוח")}, // Yad2 "ג'יפ שטח קשוח" (rugged off-roader)
	{SUV, substr("קרוסאובר")},
	{SUV, substr("crossover")},
	{SUV, substr("ג'יפ")},
	{SUV, substr("ג׳יפ")},
	{SUV, word("SUV")},

	{Wagon, substr("סטיישן")},
	{Wagon, substr("טורר")},
	{Wagon, substr("station")},
	{Wagon, substr("touring")},
	{Wagon, substr("wagon")},
	{Wagon, word("SW")},

	{Coupe, substr("קופה")},
	{Coupe, substr("קופא")},
	{Coupe, substr("coupe")},

	{Convertible, substr("קבריולה")},
	{Convertible, substr("קבריולט")},
	{Convertible, substr("convertible")},
	{Convertible, substr("cabrio")},

	{Minivan, substr("מיניוון")},
	{Minivan, substr("מיניוואן")},
	{Minivan, substr("minivan")},
	{Minivan, word("MPV")},

	{Pickup, substr("טנדר")},
	{Pickup, substr("pickup")},
}

// yad2IDToType maps Yad2's numeric bodyType ids (from the vehicle feed) to our
// canonical body types. These ids were verified against live feed data and are
// stable, so they back up text matching when Yad2 uses a term we don't yet
// recognize. Ids 5 and 8–13 have not been observed; text matching covers them.
var yad2IDToType = map[int]string{
	1:  Sedan,     // סדאן
	2:  Hatchback, // האצ'בק
	3:  Wagon,     // סטיישן / טורר
	4:  Coupe,     // קופה
	6:  SUV,       // ג'יפ שטח קשוח
	7:  SUV,       // פנאי-שטח
	14: Hatchback, // ליפטבק (liftback)
}

// FromYad2 classifies a listing from Yad2's own bodyType field (id + Hebrew
// text). Yad2 sets this attribute editorially, so it is far more reliable than
// guessing from free-text sub-model names — prefer it.
//
// Text is matched first because the Hebrew text is identical across the search
// feed and the item page, whereas numeric ids are only verified for the feed.
// The id map is a fallback for terms our vocabulary doesn't recognize yet.
// Returns "" when neither yields a match (caller should fall back to sub-model).
func FromYad2(id int, text string) string {
	if bt := matchText(text); bt != "" {
		return bt
	}
	return yad2IDToType[id]
}

// Parse scans free-text strings (e.g. sub-model names) for body-type keywords,
// returning the first match. Use this only as a fallback when Yad2's structured
// bodyType field is unavailable; prefer FromYad2.
func Parse(texts ...string) string {
	for _, text := range texts {
		if bt := matchText(text); bt != "" {
			return bt
		}
	}
	return ""
}

func matchText(text string) string {
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, m := range matchers {
		if m.match(lower) {
			return m.bodyType
		}
	}
	return ""
}

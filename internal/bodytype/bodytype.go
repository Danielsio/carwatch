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
// Implemented as a switch (not a package-level map) so the lookup is immutable.
func yad2IDToType(id int) string {
	switch id {
	case 1:
		return Sedan // סדאן
	case 2:
		return Hatchback // האצ'בק
	case 3:
		return Wagon // סטיישן / טורר
	case 4:
		return Coupe // קופה
	case 6:
		return SUV // ג'יפ שטח קשוח
	case 7:
		return SUV // פנאי-שטח
	case 14:
		return Hatchback // ליפטבק (liftback)
	default:
		return ""
	}
}

// FromYad2 classifies a listing from Yad2's own bodyType field (id + texts).
// Yad2 sets this attribute editorially, so it is far more reliable than guessing
// from free-text sub-model names — prefer it.
//
// Pass every text variant Yad2 provides (e.g. english_text, textEng, Hebrew
// text); they are matched in order. Text is tried before the numeric id because
// the same words appear on both the search feed and the item page, whereas ids
// are only verified for the feed. The id is a fallback for terms our vocabulary
// doesn't recognize yet. Returns "" when Yad2 provides no recognizable body
// type — the caller must leave body_type empty rather than guess from free text.
func FromYad2(id int, texts ...string) string {
	for _, t := range texts {
		if bt := matchText(t); bt != "" {
			return bt
		}
	}
	return yad2IDToType(id)
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

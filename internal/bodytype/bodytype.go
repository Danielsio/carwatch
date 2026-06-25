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
	{Hatchback, substr("hatchback")},
	{Hatchback, word("HB")},

	{Sedan, substr("סדאן")},
	{Sedan, substr("sedan")},

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
	{Minivan, substr("minivan")},
	{Minivan, word("MPV")},

	{Pickup, substr("טנדר")},
	{Pickup, substr("pickup")},
}

func Parse(texts ...string) string {
	for _, text := range texts {
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		for _, m := range matchers {
			if m.match(lower) {
				return m.bodyType
			}
		}
	}
	return ""
}

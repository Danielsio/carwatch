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

var All = []string{Sedan, Hatchback, SUV, Coupe, Wagon, Convertible, Minivan, Pickup}

type matcher struct {
	bodyType string
	match    func(lower string) bool
}

func contains(substr string) func(string) bool {
	return func(s string) bool { return strings.Contains(s, substr) }
}

func wordBoundary(word string) func(string) bool {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	return func(s string) bool { return re.MatchString(s) }
}

var matchers []matcher

func init() {
	matchers = []matcher{
		// Hatchback (before sedan — more specific)
		{Hatchback, contains("האצ'בק")},
		{Hatchback, contains("האצ׳בק")}, // modified letter geresh ׳
		{Hatchback, contains("hatchback")},
		{Hatchback, wordBoundary("HB")},

		// Sedan
		{Sedan, contains("סדאן")},
		{Sedan, contains("sedan")},

		// SUV / Crossover
		{SUV, contains("קרוסאובר")},
		{SUV, contains("crossover")},
		{SUV, contains("ג'יפ")},
		{SUV, contains("ג׳יפ")}, // modified letter geresh ׳
		{SUV, wordBoundary("SUV")},

		// Wagon / Station / Touring
		{Wagon, contains("סטיישן")},
		{Wagon, contains("טורר")},
		{Wagon, contains("station")},
		{Wagon, contains("touring")},
		{Wagon, contains("wagon")},
		{Wagon, wordBoundary("SW")},

		// Coupe
		{Coupe, contains("קופה")},
		{Coupe, contains("קופא")},
		{Coupe, contains("coupe")},

		// Convertible
		{Convertible, contains("קבריולה")},
		{Convertible, contains("קבריולט")},
		{Convertible, contains("convertible")},
		{Convertible, contains("cabrio")},

		// Minivan
		{Minivan, contains("מיניוון")},
		{Minivan, contains("minivan")},
		{Minivan, wordBoundary("MPV")},

		// Pickup
		{Pickup, contains("טנדר")},
		{Pickup, contains("pickup")},
	}
}

func Parse(subModel string) string {
	if subModel == "" {
		return ""
	}
	lower := strings.ToLower(subModel)
	for _, m := range matchers {
		if m.match(lower) {
			return m.bodyType
		}
	}
	return ""
}

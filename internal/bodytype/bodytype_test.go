package bodytype

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		subModel string
		want     string
	}{
		// Hebrew body types
		{"hebrew sedan", "Comfort סדאן 1.6 (132 כ״ס)", "sedan"},
		{"hebrew hatchback geresh", "Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)", "hatchback"},
		{"hebrew hatchback modified", "Excite האצ׳בק 1.6", "hatchback"},
		{"hebrew suv jeep geresh", "Limited ג'יפ 2.0 (150 כ״ס)", "suv"},
		{"hebrew suv jeep modified", "Sport ג׳יפ 2.5", "suv"},
		{"hebrew crossover", "Active קרוסאובר 1.5 (115 כ״ס)", "suv"},
		{"hebrew wagon station", "Touring סטיישן 2.0 (150 כ״ס)", "wagon"},
		{"hebrew wagon tourer", "Luxury טורר אוט׳ 1.8", "wagon"},
		{"hebrew coupe", "Sport קופה 2.0 (200 כ״ס)", "coupe"},
		{"hebrew coupe alt", "RS קופא 3.0", "coupe"},
		{"hebrew convertible cabrio", "Sport קבריולה 2.0", "convertible"},
		{"hebrew convertible cabriolet", "קבריולט 1.8", "convertible"},
		{"hebrew minivan", "Comfort מיניוון 2.0 (150 כ״ס)", "minivan"},
		{"hebrew minivan alt spelling", "Comfort מיניוואן 2.0", "minivan"},
		{"hebrew pickup", "Limited טנדר 3.0 (190 כ״ס)", "pickup"},

		// Yad2 official bodyType vocabulary (previously unmatched → fell back to
		// sub-model parsing). These are the values from the structured field.
		{"yad2 crossover leisure", "פנאי-שטח", "suv"},
		{"yad2 crossover leisure spaced", "פנאי שטח", "suv"},
		{"yad2 rugged jeep", "ג'יפ שטח קשוח", "suv"},
		{"yad2 liftback", "ליפטבק", "hatchback"},
		{"yad2 station tourer", "סטיישן / טורר", "wagon"},

		// English body types
		{"english sedan", "Comfort sedan 1.6", "sedan"},
		{"english hatchback", "Sport hatchback 1.4 TSI", "hatchback"},
		{"english HB word boundary", "Sport HB 1.4", "hatchback"},
		{"english SUV word boundary", "Premium SUV 2.0", "suv"},
		{"english crossover", "Active crossover 1.5", "suv"},
		{"english wagon", "Touring wagon 2.0", "wagon"},
		{"english SW word boundary", "Comfort SW 1.6", "wagon"},
		{"english station", "station 2.0 diesel", "wagon"},
		{"english touring", "Grand Touring 2.5", "wagon"},
		{"english coupe", "Sport coupe 2.0", "coupe"},
		{"english convertible", "Sport convertible 2.0", "convertible"},
		{"english cabrio", "Sport cabrio 1.8", "convertible"},
		{"english minivan", "Comfort minivan 2.4", "minivan"},
		{"english MPV word boundary", "Comfort MPV 2.0", "minivan"},
		{"english pickup", "Limited pickup 3.0", "pickup"},

		// Case insensitivity
		{"mixed case SEDAN", "Comfort SEDAN 1.6", "sedan"},
		{"mixed case Hatchback", "Sport Hatchback 1.4", "hatchback"},
		{"lowercase suv", "Premium suv 2.0", "suv"},

		// Word boundary: short tokens must not match inside words
		{"SW inside word SKYVIEW", "Premium SKYVIEW 2.0", ""},
		{"HB inside word THBIRD", "Premium THBIRD 1.8", ""},
		{"SUV inside word INSUVERABLE", "INSUVERABLE 2.0", ""},

		// No match
		{"empty string", "", ""},
		{"no body type keywords", "Premium אוט׳ 2.0 (165 כ״ס) [2017-2019]", ""},
		{"just trim info", "LUXURY", ""},
		{"engine only", "1.8 (98 כ״ס)", ""},

		// Real Yad2 submodel strings
		{"real yad2 hatchback", "Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8 (98 כ״ס)", "hatchback"},
		{"real yad2 no body type", "Premium אוט׳ 2.0 (165 כ״ס) [2017-2019]", ""},
		{"real yad2 sport no type", "Sport אוט׳ 1.8", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.subModel)
			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.subModel, got, tt.want)
			}
		})
	}
}

func TestFromYad2(t *testing.T) {
	tests := []struct {
		name string
		id   int
		text string
		want string
	}{
		// Verified Yad2 feed ids → canonical type (text matched too).
		{"id 1 sedan", 1, "סדאן", "sedan"},
		{"id 2 hatchback", 2, "האצ'בק", "hatchback"},
		{"id 3 wagon", 3, "סטיישן / טורר", "wagon"},
		{"id 4 coupe", 4, "קופה", "coupe"},
		{"id 6 rugged suv", 6, "ג'יפ שטח קשוח", "suv"},
		{"id 7 crossover suv", 7, "פנאי-שטח", "suv"},
		{"id 14 liftback", 14, "ליפטבק", "hatchback"},

		// Text matches our vocabulary even when the id is unknown.
		{"unknown id known text", 99, "סדאן", "sedan"},

		// Unknown text but a verified id still classifies (defense-in-depth for
		// a future Yad2 wording change).
		{"known id unrecognized text", 7, "מילה חדשה", "suv"},

		// Neither resolves → empty (caller falls back to sub-model parsing).
		{"unknown id and text", 0, "", ""},
		{"unknown id unknown text", 99, "משהו אחר", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromYad2(tt.id, tt.text)
			if got != tt.want {
				t.Errorf("FromYad2(%d, %q) = %q, want %q", tt.id, tt.text, got, tt.want)
			}
		})
	}
}

func TestParse_MultipleTexts(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
		want  string
	}{
		{"english hit", []string{"Sport hatchback 1.4", "ספורט האצ'בק 1.4"}, "hatchback"},
		{"english miss falls back to hebrew", []string{"Excite Plus", "Excite Plus היברידי אוט׳ האצ'בק 5 דל 1.8"}, "hatchback"},
		{"both empty", []string{"", ""}, ""},
		{"first empty second hit", []string{"", "סדאן 1.6"}, "sedan"},
		{"single text", []string{"SUV Premium"}, "suv"},
		{"no match in any", []string{"Excite Plus", "Premium אוט׳ 2.0"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.texts...)
			if got != tt.want {
				t.Errorf("Parse(%v) = %q, want %q", tt.texts, got, tt.want)
			}
		})
	}
}

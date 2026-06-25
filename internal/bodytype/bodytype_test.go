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
		{"hebrew pickup", "Limited טנדר 3.0 (190 כ״ס)", "pickup"},

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

package bodytype

import "testing"

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

		// Official Yad2 labels for types without a verified id are matched by
		// text (the only remaining classification path — no sub-model guessing).
		{"convertible label", 0, "קבריולה", "convertible"},
		{"minivan label", 0, "מיניוואן", "minivan"},
		{"pickup label", 0, "טנדר", "pickup"},

		// Text matches our vocabulary even when the id is unknown.
		{"unknown id known text", 99, "סדאן", "sedan"},

		// Unknown text but a verified id still classifies (defense-in-depth for
		// a future Yad2 wording change).
		{"known id unrecognized text", 7, "מילה חדשה", "suv"},

		// Neither the id nor the label resolves → empty. body_type stays empty;
		// we never fall back to guessing from the sub-model.
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

func TestFromYad2_MatchesAcrossTexts(t *testing.T) {
	// English variant present, Hebrew empty — must still classify.
	if got := FromYad2(0, "Sedan", ""); got != "sedan" {
		t.Errorf(`FromYad2(0, "Sedan", "") = %q, want "sedan"`, got)
	}
	// First text unrecognized, later text matches.
	if got := FromYad2(0, "weird-value", "פנאי-שטח"); got != "suv" {
		t.Errorf(`FromYad2(0, "weird-value", "פנאי-שטח") = %q, want "suv"`, got)
	}
	// No text matches but a verified id does (defense-in-depth).
	if got := FromYad2(7, "weird-value", ""); got != "suv" {
		t.Errorf(`FromYad2(7, "weird-value", "") = %q, want "suv"`, got)
	}
}

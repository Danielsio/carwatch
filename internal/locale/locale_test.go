package locale

import "testing"

func TestT(t *testing.T) {
	got := T(Hebrew, "welcome")
	if got == "" || got == "welcome" {
		t.Fatalf("expected Hebrew welcome, got %q", got)
	}

	got = T(English, "welcome")
	if got == "" || got == "welcome" {
		t.Fatalf("expected English welcome, got %q", got)
	}
}

func TestTFallback(t *testing.T) {
	got := T("xx", "welcome")
	if got != T(English, "welcome") {
		t.Fatal("expected English fallback for unknown lang")
	}
}

func TestTMissingKey(t *testing.T) {
	got := T(Hebrew, "nonexistent_key_xyz")
	if got != "nonexistent_key_xyz" {
		t.Fatalf("expected key returned as-is for missing key, got %q", got)
	}
}

func TestTf(t *testing.T) {
	got := Tf(English, "stop_success", 42)
	want := "Search #42 deleted."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAllEnglishKeysHaveHebrew(t *testing.T) {
	he := mergeMaps(heWizard, heCommands, heScoring)
	en := mergeMaps(enWizard, enCommands, enScoring)
	for key := range en {
		if _, ok := he[key]; !ok {
			t.Errorf("English key %q missing from Hebrew translations", key)
		}
	}
}

func TestAllHebrewKeysHaveEnglish(t *testing.T) {
	he := mergeMaps(heWizard, heCommands, heScoring)
	en := mergeMaps(enWizard, enCommands, enScoring)
	for key := range he {
		if _, ok := en[key]; !ok {
			t.Errorf("Hebrew key %q missing from English translations", key)
		}
	}
}

func TestMergeMaps(t *testing.T) {
	a := map[string]string{"x": "1"}
	b := map[string]string{"y": "2"}
	m := mergeMaps(a, b)
	if len(m) != 2 || m["x"] != "1" || m["y"] != "2" {
		t.Fatalf("mergeMaps: %v", m)
	}
}

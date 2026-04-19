package bot

import (
	"encoding/json"
	"testing"
)

func TestWizardData_JSON(t *testing.T) {
	wd := WizardData{
		Manufacturer:     27,
		ManufacturerName: "Mazda",
		Model:            10332,
		ModelName:        "3",
		YearMin:          2018,
		YearMax:          2024,
		PriceMax:         150000,
		EngineMinCC:      2000,
	}

	data, err := json.Marshal(wd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded WizardData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Manufacturer != 27 || decoded.ModelName != "3" || decoded.PriceMax != 150000 {
		t.Errorf("roundtrip failed: %+v", decoded)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{85000, "85,000"},
		{150000, "150,000"},
		{1500000, "1,500,000"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestKeyboards_ManufacturerKeyboard(t *testing.T) {
	kb := manufacturerKeyboard()
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("keyboard should have rows")
	}

	total := 0
	for _, row := range kb.InlineKeyboard {
		total += len(row)
		for _, btn := range row {
			if btn.CallbackData == "" {
				t.Errorf("button %q has empty callback data", btn.Text)
			}
		}
	}
	if total < 15 {
		t.Errorf("expected at least 15 manufacturer buttons, got %d", total)
	}
}

func TestKeyboards_ModelKeyboard(t *testing.T) {
	kb := modelKeyboard(27) // Mazda
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("Mazda model keyboard should have rows")
	}

	found := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "3" {
				found = true
			}
		}
	}
	if !found {
		t.Error("Mazda 3 button not found in model keyboard")
	}
}

func TestKeyboards_EngineKeyboard(t *testing.T) {
	kb := engineKeyboard()
	if len(kb.InlineKeyboard) != 2 {
		t.Errorf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
}

func TestKeyboards_ConfirmKeyboard(t *testing.T) {
	wd := WizardData{
		ManufacturerName: "Mazda",
		ModelName:        "3",
		YearMin:          2018,
		YearMax:          2024,
		PriceMax:         150000,
		EngineMinCC:      2000,
	}

	kb, summary := confirmKeyboard(wd)
	if kb == nil {
		t.Fatal("keyboard should not be nil")
	}
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
	if !contains(summary, "Mazda") || !contains(summary, "150,000") {
		t.Errorf("summary missing expected content: %s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

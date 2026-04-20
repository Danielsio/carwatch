package yad2

import "testing"

func TestManufacturers(t *testing.T) {
	mfrs := Manufacturers()
	if len(mfrs) < 10 {
		t.Errorf("expected at least 10 manufacturers, got %d", len(mfrs))
	}

	found := false
	for _, m := range mfrs {
		if m.ID == 27 && m.Name == "Mazda" {
			found = true
		}
	}
	if !found {
		t.Error("Mazda (ID 27) not found in catalog")
	}
}

func TestModels(t *testing.T) {
	models := Models(27)
	if len(models) == 0 {
		t.Fatal("no models for Mazda (27)")
	}

	found := false
	for _, m := range models {
		if m.ID == 10332 && m.Name == "3" {
			found = true
		}
	}
	if !found {
		t.Error("Mazda 3 (ID 10332) not found")
	}
}

func TestModels_UnknownManufacturer(t *testing.T) {
	models := Models(99999)
	if models != nil {
		t.Errorf("expected nil for unknown manufacturer, got %d models", len(models))
	}
}

func TestManufacturerName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{27, "Mazda"},
		{35, "Toyota"},
		{99999, "Unknown"},
	}
	for _, tt := range tests {
		got := ManufacturerName(tt.id)
		if got != tt.want {
			t.Errorf("ManufacturerName(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestModelName(t *testing.T) {
	tests := []struct {
		mfr, model int
		want       string
	}{
		{27, 10332, "3"},
		{35, 10471, "Corolla"},
		{27, 99999, "Unknown"},
	}
	for _, tt := range tests {
		got := ModelName(tt.mfr, tt.model)
		if got != tt.want {
			t.Errorf("ModelName(%d, %d) = %q, want %q", tt.mfr, tt.model, got, tt.want)
		}
	}
}

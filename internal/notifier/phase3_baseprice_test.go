package notifier

import (
	"strings"
	"testing"

	"github.com/dsionov/carwatch/internal/locale"
)

func TestBasePriceLine_BelowBase(t *testing.T) {
	line := basePriceLine(locale.Hebrew, 100000, 80000)
	if line == "" {
		t.Fatal("expected non-empty line for price well below base")
	}
	if !strings.Contains(line, "20") {
		t.Errorf("expected ~20%% below mention, got: %s", line)
	}
}

func TestBasePriceLine_NearBase(t *testing.T) {
	line := basePriceLine(locale.Hebrew, 100000, 98000)
	if line == "" {
		t.Fatal("expected non-empty line for price near base")
	}
	if !strings.Contains(line, "100,000") {
		t.Errorf("near-base line should mention the base price, got: %s", line)
	}
}

func TestBasePriceLine_AboveBase(t *testing.T) {
	line := basePriceLine(locale.Hebrew, 100000, 120000)
	if line == "" {
		t.Fatal("expected non-empty line for price well above base")
	}
	if !strings.Contains(line, "20") {
		t.Errorf("expected ~20%% above mention, got: %s", line)
	}
}

func TestBasePriceLine_English(t *testing.T) {
	below := basePriceLine(locale.English, 100000, 80000)
	near := basePriceLine(locale.English, 100000, 100000)
	above := basePriceLine(locale.English, 100000, 120000)

	if below == "" || near == "" || above == "" {
		t.Error("expected non-empty lines for all English cases")
	}
}

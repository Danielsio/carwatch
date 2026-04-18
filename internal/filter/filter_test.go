package filter

import (
	"testing"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

func TestApply_EngineFilter(t *testing.T) {
	listings := []model.RawListing{
		{Token: "a", EngineVolume: 1600},
		{Token: "b", EngineVolume: 1998},
		{Token: "c", EngineVolume: 2500},
	}

	criteria := config.FilterCriteria{
		EngineMinCC: 1800,
		EngineMaxCC: 2100,
	}

	result := Apply(criteria, listings)
	if len(result) != 1 || result[0].Token != "b" {
		t.Errorf("expected 1 listing (token=b), got %d", len(result))
	}
}

func TestApply_MaxKm(t *testing.T) {
	listings := []model.RawListing{
		{Token: "a", Km: 50000},
		{Token: "b", Km: 200000},
	}

	criteria := config.FilterCriteria{MaxKm: 150000}

	result := Apply(criteria, listings)
	if len(result) != 1 || result[0].Token != "a" {
		t.Errorf("expected 1 listing (token=a), got %d", len(result))
	}
}

func TestApply_MaxHand(t *testing.T) {
	listings := []model.RawListing{
		{Token: "a", Hand: 2},
		{Token: "b", Hand: 5},
	}

	criteria := config.FilterCriteria{MaxHand: 3}

	result := Apply(criteria, listings)
	if len(result) != 1 || result[0].Token != "a" {
		t.Errorf("expected 1 listing (token=a), got %d", len(result))
	}
}

func TestApply_Keywords(t *testing.T) {
	listings := []model.RawListing{
		{Token: "a", Description: "Great car with sunroof"},
		{Token: "b", Description: "Basic model"},
	}

	criteria := config.FilterCriteria{Keywords: []string{"sunroof"}}

	result := Apply(criteria, listings)
	if len(result) != 1 || result[0].Token != "a" {
		t.Errorf("expected 1 listing (token=a), got %d", len(result))
	}
}

func TestApply_ExcludeKeys(t *testing.T) {
	listings := []model.RawListing{
		{Token: "a", Description: "Accident free car"},
		{Token: "b", Description: "Minor accident damage"},
	}

	criteria := config.FilterCriteria{ExcludeKeys: []string{"accident damage"}}

	result := Apply(criteria, listings)
	if len(result) != 1 || result[0].Token != "a" {
		t.Errorf("expected 1 listing (token=a), got %d", len(result))
	}
}

func TestApply_ZeroValuesDisableFilter(t *testing.T) {
	listings := []model.RawListing{
		{Token: "a", EngineVolume: 1600, Km: 200000, Hand: 5},
	}

	criteria := config.FilterCriteria{}

	result := Apply(criteria, listings)
	if len(result) != 1 {
		t.Errorf("expected all listings to pass with zero criteria, got %d", len(result))
	}
}

func TestApply_CombinedFilters(t *testing.T) {
	listings := []model.RawListing{
		{Token: "a", EngineVolume: 1998, Km: 80000, Hand: 2, Description: "clean car"},
		{Token: "b", EngineVolume: 1998, Km: 200000, Hand: 2, Description: "clean car"},
		{Token: "c", EngineVolume: 1600, Km: 80000, Hand: 2, Description: "clean car"},
		{Token: "d", EngineVolume: 1998, Km: 80000, Hand: 5, Description: "clean car"},
	}

	criteria := config.FilterCriteria{
		EngineMinCC: 1800,
		EngineMaxCC: 2100,
		MaxKm:     150000,
		MaxHand:   3,
	}

	result := Apply(criteria, listings)
	if len(result) != 1 || result[0].Token != "a" {
		t.Errorf("expected 1 listing (token=a), got %d", len(result))
	}
}

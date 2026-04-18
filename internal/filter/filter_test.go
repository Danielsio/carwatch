package filter

import (
	"testing"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

func TestApply(t *testing.T) {
	tests := []struct {
		name     string
		criteria config.FilterCriteria
		listings []model.RawListing
		want     []string // expected tokens
	}{
		{
			name:     "engine min filter",
			criteria: config.FilterCriteria{EngineMinCC: 1800},
			listings: []model.RawListing{
				{Token: "a", EngineVolume: 1600},
				{Token: "b", EngineVolume: 1998},
			},
			want: []string{"b"},
		},
		{
			name:     "engine max filter",
			criteria: config.FilterCriteria{EngineMaxCC: 2100},
			listings: []model.RawListing{
				{Token: "a", EngineVolume: 1998},
				{Token: "b", EngineVolume: 2500},
			},
			want: []string{"a"},
		},
		{
			name:     "engine range filter",
			criteria: config.FilterCriteria{EngineMinCC: 1800, EngineMaxCC: 2100},
			listings: []model.RawListing{
				{Token: "a", EngineVolume: 1600},
				{Token: "b", EngineVolume: 1998},
				{Token: "c", EngineVolume: 2500},
			},
			want: []string{"b"},
		},
		{
			name:     "max km filter",
			criteria: config.FilterCriteria{MaxKm: 150000},
			listings: []model.RawListing{
				{Token: "a", Km: 50000},
				{Token: "b", Km: 200000},
			},
			want: []string{"a"},
		},
		{
			name:     "max hand filter",
			criteria: config.FilterCriteria{MaxHand: 3},
			listings: []model.RawListing{
				{Token: "a", Hand: 2},
				{Token: "b", Hand: 5},
			},
			want: []string{"a"},
		},
		{
			name:     "keyword inclusion",
			criteria: config.FilterCriteria{Keywords: []string{"sunroof"}},
			listings: []model.RawListing{
				{Token: "a", Description: "Great car with sunroof"},
				{Token: "b", Description: "Basic model"},
			},
			want: []string{"a"},
		},
		{
			name:     "keyword exclusion",
			criteria: config.FilterCriteria{ExcludeKeys: []string{"accident damage"}},
			listings: []model.RawListing{
				{Token: "a", Description: "Accident free car"},
				{Token: "b", Description: "Minor accident damage"},
			},
			want: []string{"a"},
		},
		{
			name:     "keyword case insensitive",
			criteria: config.FilterCriteria{Keywords: []string{"SUNROOF"}},
			listings: []model.RawListing{
				{Token: "a", Description: "has a sunroof"},
			},
			want: []string{"a"},
		},
		{
			name:     "zero values disable all filters",
			criteria: config.FilterCriteria{},
			listings: []model.RawListing{
				{Token: "a", EngineVolume: 1600, Km: 200000, Hand: 5},
			},
			want: []string{"a"},
		},
		{
			name: "combined filters",
			criteria: config.FilterCriteria{
				EngineMinCC: 1800,
				EngineMaxCC: 2100,
				MaxKm:       150000,
				MaxHand:     3,
			},
			listings: []model.RawListing{
				{Token: "pass", EngineVolume: 1998, Km: 80000, Hand: 2, Description: "clean"},
				{Token: "fail-km", EngineVolume: 1998, Km: 200000, Hand: 2, Description: "clean"},
				{Token: "fail-engine", EngineVolume: 1600, Km: 80000, Hand: 2, Description: "clean"},
				{Token: "fail-hand", EngineVolume: 1998, Km: 80000, Hand: 5, Description: "clean"},
			},
			want: []string{"pass"},
		},
		{
			name:     "empty listings",
			criteria: config.FilterCriteria{MaxKm: 100000},
			listings: []model.RawListing{},
			want:     []string{},
		},
		{
			name: "multiple keywords all required",
			criteria: config.FilterCriteria{Keywords: []string{"sunroof", "leather"}},
			listings: []model.RawListing{
				{Token: "a", Description: "sunroof and leather seats"},
				{Token: "b", Description: "has sunroof only"},
				{Token: "c", Description: "leather only"},
			},
			want: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Apply(tt.criteria, tt.listings)
			got := make([]string, len(result))
			for i, l := range result {
				got[i] = l.Token
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d listings %v, want %d %v", len(got), got, len(tt.want), tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("listing[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsionov/carwatch/internal/fetcher"
)

// The UI hides its refresh / instant-search entry points when live_search is
// false, so this flag must track whether the fetchers are actually wired.
func TestCapabilities_ReportsLiveSearch(t *testing.T) {
	tests := []struct {
		name     string
		fetchers bool
		want     bool
	}{
		{"no fetchers (server-side fetch disabled)", false, false},
		{"fetchers wired", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{}
			if tt.fetchers {
				s.fetchers = fetcher.NewFactory()
			}

			rec := httptest.NewRecorder()
			s.capabilities(rec, httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var got capabilitiesResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.LiveSearch != tt.want {
				t.Errorf("live_search = %v, want %v", got.LiveSearch, tt.want)
			}
		})
	}
}

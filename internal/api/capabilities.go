package api

import "net/http"

// capabilitiesResponse tells the web app which optional features this
// deployment can actually perform, so the UI can hide the ones it cannot
// rather than offering a button that only ever returns an error.
type capabilitiesResponse struct {
	// LiveSearch reports whether the server can fetch listings from the source
	// on demand. It backs the per-search refresh and the guest instant search.
	//
	// False whenever server-side fetching is off (see
	// config.PollingConfig.ServerFetch): Yad2 serves its pages behind a bot
	// challenge only a real browser clears, so those fetches cannot succeed and
	// the handlers answer 503. Listings still arrive — via the browser
	// extension's /ext/ingest — so the rest of the app is unaffected.
	LiveSearch bool `json:"live_search"`
}

func (s *Server) capabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, capabilitiesResponse{
		LiveSearch: s.fetchers != nil,
	})
}

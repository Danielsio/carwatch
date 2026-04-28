package api

import (
	"net/http"
	"strconv"
)

type catalogEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (s *Server) listManufacturers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	var entries []catalogEntry
	if q != "" {
		for _, e := range s.catalog.SearchManufacturers(q) {
			entries = append(entries, catalogEntry{ID: e.ID, Name: e.Name})
		}
	} else {
		for _, e := range s.catalog.Manufacturers() {
			entries = append(entries, catalogEntry{ID: e.ID, Name: e.Name})
		}
	}

	if entries == nil {
		entries = []catalogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	mfrID, err := strconv.Atoi(idStr)
	if err != nil || mfrID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid manufacturer id")
		return
	}

	q := r.URL.Query().Get("q")

	var entries []catalogEntry
	if q != "" {
		for _, e := range s.catalog.SearchModels(mfrID, q) {
			entries = append(entries, catalogEntry{ID: e.ID, Name: e.Name})
		}
	} else {
		for _, e := range s.catalog.Models(mfrID) {
			entries = append(entries, catalogEntry{ID: e.ID, Name: e.Name})
		}
	}

	if entries == nil {
		entries = []catalogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

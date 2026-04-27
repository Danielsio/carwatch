package api

import (
	"net/http"
)

type listingResponse struct {
	Token        string   `json:"token"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	Year         int      `json:"year"`
	Price        int      `json:"price"`
	Km           int      `json:"km"`
	Hand         int      `json:"hand"`
	City         string   `json:"city"`
	PageLink     string   `json:"page_link"`
	ImageURL     string   `json:"image_url,omitempty"`
	FitnessScore *float64 `json:"fitness_score,omitempty"`
	FirstSeenAt  string   `json:"first_seen_at"`
}

type listingsPageResponse struct {
	Items  []listingResponse `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

func (s *Server) listListings(w http.ResponseWriter, r *http.Request) {
	chatID := chatIDFromContext(r.Context())
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid search id")
		return
	}

	sr, err := s.searches.GetSearch(r.Context(), id)
	if err != nil {
		s.logger.Error("get search for listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get search")
		return
	}
	if sr == nil || sr.ChatID != chatID {
		writeError(w, http.StatusNotFound, "search not found")
		return
	}

	limit := parseIntParam(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := parseIntParam(r, "offset", 0)
	sort := parseSortParam(r)

	listings, err := s.listings.ListSearchListings(r.Context(), chatID, sr.Name, limit, offset, sort)
	if err != nil {
		s.logger.Error("list search listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list listings")
		return
	}

	total, err := s.listings.CountSearchListings(r.Context(), chatID, sr.Name)
	if err != nil {
		s.logger.Error("count search listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count listings")
		return
	}

	items := make([]listingResponse, 0, len(listings))
	for _, l := range listings {
		items = append(items, listingResponse{
			Token:        l.Token,
			Manufacturer: l.Manufacturer,
			Model:        l.Model,
			Year:         l.Year,
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			City:         l.City,
			PageLink:     l.PageLink,
			ImageURL:     l.ImageURL,
			FitnessScore: l.FitnessScore,
			FirstSeenAt:  l.FirstSeenAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, http.StatusOK, listingsPageResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

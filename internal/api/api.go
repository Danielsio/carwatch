package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
)

type contextKey string

const chatIDKey contextKey = "chatID"

type Server struct {
	catalog  catalog.Catalog
	searches storage.SearchStore
	listings storage.ListingStore
	users    storage.UserStore
	prices   storage.PriceTracker
	logger   *slog.Logger
	cfg      config.APIConfig
}

type Config struct {
	Catalog  catalog.Catalog
	Searches storage.SearchStore
	Listings storage.ListingStore
	Users    storage.UserStore
	Prices   storage.PriceTracker
	Logger   *slog.Logger
	API      config.APIConfig
}

func New(c Config) *Server {
	return &Server{
		catalog:  c.Catalog,
		searches: c.Searches,
		listings: c.Listings,
		users:    c.Users,
		prices:   c.Prices,
		logger:   c.Logger,
		cfg:      c.API,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/catalog/manufacturers", s.listManufacturers)
	mux.HandleFunc("GET /api/v1/catalog/manufacturers/{id}/models", s.listModels)

	mux.HandleFunc("GET /api/v1/searches", s.listSearches)
	mux.HandleFunc("POST /api/v1/searches", s.createSearch)
	mux.HandleFunc("GET /api/v1/searches/{id}", s.getSearch)
	mux.HandleFunc("PUT /api/v1/searches/{id}", s.updateSearch)
	mux.HandleFunc("DELETE /api/v1/searches/{id}", s.deleteSearch)
	mux.HandleFunc("POST /api/v1/searches/{id}/pause", s.pauseSearch)
	mux.HandleFunc("POST /api/v1/searches/{id}/resume", s.resumeSearch)

	mux.HandleFunc("GET /api/v1/searches/{id}/listings", s.listListings)

	return s.corsMiddleware(s.authMiddleware(mux))
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	origins := make(map[string]bool, len(s.cfg.CORSOrigins))
	for _, o := range s.cfg.CORSOrigins {
		origins[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatID int64

		if s.cfg.AuthToken != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+s.cfg.AuthToken {
				writeError(w, http.StatusUnauthorized, "invalid or missing token")
				return
			}
		}

		chatID = s.cfg.DevChatID
		if chatID == 0 {
			writeError(w, http.StatusUnauthorized, "no user configured")
			return
		}

		ctx := context.WithValue(r.Context(), chatIDKey, chatID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func chatIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(chatIDKey).(int64)
	return id
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("failed to write JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseIntParam(r *http.Request, name string, defaultVal int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}

func parsePathID(r *http.Request) (int64, bool) {
	s := r.PathValue("id")
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseSortParam(r *http.Request) string {
	s := r.URL.Query().Get("sort")
	switch s {
	case "price_asc", "price_desc", "score", "km", "year":
		return s
	default:
		return "newest"
	}
}

func splitKeywords(s string) string {
	return strings.TrimSpace(s)
}

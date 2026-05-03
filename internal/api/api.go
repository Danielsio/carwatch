package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	fbauth "firebase.google.com/go/v4/auth"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/storage"
)

type contextKey string

const (
	chatIDKey contextKey = "chatID"
	emailKey  contextKey = "email"
)

type PollTrigger interface {
	TriggerPoll()
}

type Server struct {
	catalog       catalog.Catalog
	searches      storage.SearchStore
	listings      storage.ListingStore
	users         storage.UserStore
	linkTokens    storage.LinkTokenStore
	firebaseAuth  TokenVerifier
	prices        storage.PriceTracker
	admin     storage.AdminStore
	saved     storage.SavedListingStore
	hidden    storage.HiddenListingStore
	notifs    storage.NotificationStore
	poller    PollTrigger
	logger    *slog.Logger
	cfg       config.APIConfig
	botUsername string
	startTime time.Time
	rl        *rateLimiter
	vacuumMu  sync.Mutex
}

func (s *Server) SetPollTrigger(p PollTrigger) {
	s.poller = p
}

type Config struct {
	Catalog  catalog.Catalog
	Searches storage.SearchStore
	Listings storage.ListingStore
	Users    storage.UserStore
	LinkTokens storage.LinkTokenStore
	Prices   storage.PriceTracker
	Admin    storage.AdminStore
	Saved    storage.SavedListingStore
	Hidden       storage.HiddenListingStore
	Notifs       storage.NotificationStore
	Logger       *slog.Logger
	API          config.APIConfig
	FirebaseAuth TokenVerifier
	BotUsername  string
}

func New(c Config) *Server {
	return &Server{
		catalog:      c.Catalog,
		searches:     c.Searches,
		listings:     c.Listings,
		users:        c.Users,
		linkTokens:   c.LinkTokens,
		firebaseAuth: c.FirebaseAuth,
		prices:       c.Prices,
		admin:     c.Admin,
		saved:     c.Saved,
		hidden:    c.Hidden,
		notifs:    c.Notifs,
		logger:    c.Logger,
		cfg:       c.API,
		botUsername: c.BotUsername,
		startTime: time.Now(),
		rl:        newRateLimiter(context.Background(), 60, time.Second/60),
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
	mux.HandleFunc("GET /api/v1/listings/{token}", s.getListing)

	if s.admin != nil {
		mux.HandleFunc("GET /api/v1/admin/stats", s.requireAdmin(s.adminStats))
		mux.HandleFunc("GET /api/v1/admin/listings", s.requireAdmin(s.adminListListings))
		mux.HandleFunc("DELETE /api/v1/admin/listings/{token}", s.requireAdmin(s.adminDeleteListing))
		mux.HandleFunc("POST /api/v1/admin/purge", s.requireAdmin(s.adminPurgeTable))
		mux.HandleFunc("POST /api/v1/admin/vacuum", s.requireAdmin(s.adminVacuum))
	}

	if s.notifs != nil {
		mux.HandleFunc("GET /api/v1/notifications", s.listNotifications)
		mux.HandleFunc("GET /api/v1/notifications/count", s.notificationCount)
		mux.HandleFunc("POST /api/v1/notifications/seen", s.markNotificationsSeen)
	}

	mux.HandleFunc("GET /api/v1/telegram/status", s.getTelegramStatus)
	if s.linkTokens != nil {
		mux.HandleFunc("POST /api/v1/telegram/link", s.postTelegramLink)
	}

	if s.saved != nil && s.hidden != nil {
		mux.HandleFunc("GET /api/v1/saved", s.listSaved)
		mux.HandleFunc("POST /api/v1/listings/{token}/save", s.saveListing)
		mux.HandleFunc("DELETE /api/v1/listings/{token}/save", s.unsaveListing)
		mux.HandleFunc("POST /api/v1/listings/{token}/hide", s.hideListing)
		mux.HandleFunc("DELETE /api/v1/listings/{token}/hide", s.unhideListing)
		mux.HandleFunc("GET /api/v1/history", s.listHistory)
	}

	return s.corsMiddleware(s.authMiddleware(s.withRateLimit(mux)))
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
			w.Header().Set("Vary", "Origin")
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
		authHdr := r.Header.Get("Authorization")
		bearer := bearerFromAuthHeader(authHdr)

		var chatID int64

		var userEmail string
		if s.firebaseAuth != nil && bearer != "" {
			tok, err := s.firebaseAuth.VerifyIDToken(r.Context(), bearer)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or missing token")
				return
			}
			userEmail = emailFromClaims(tok)
			id, err := s.users.UpsertWebUser(r.Context(), tok.UID, userEmail)
			if err != nil {
				s.logger.Error("upsert web user", "error", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			chatID = id
		} else if s.firebaseAuth != nil && bearer == "" {
			writeError(w, http.StatusUnauthorized, "invalid or missing token")
			return
		} else if s.firebaseAuth == nil {
			if s.cfg.AuthToken != "" {
				if authHdr != "Bearer "+s.cfg.AuthToken {
					writeError(w, http.StatusUnauthorized, "invalid or missing token")
					return
				}
			}
			chatID = s.cfg.DevChatID
			if chatID == 0 {
				writeError(w, http.StatusUnauthorized, "no user configured")
				return
			}
		} else {
			writeError(w, http.StatusUnauthorized, "invalid or missing token")
			return
		}

		ctx := context.WithValue(r.Context(), chatIDKey, chatID)
		ctx = context.WithValue(ctx, emailKey, userEmail)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerFromAuthHeader(authHdr string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHdr, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHdr, prefix))
}

func emailFromClaims(tok *fbauth.Token) string {
	if tok == nil {
		return ""
	}
	v, ok := tok.Claims["email"]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func chatIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(chatIDKey).(int64)
	return id
}

func emailFromContext(ctx context.Context) string {
	e, _ := ctx.Value(emailKey).(string)
	return e
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

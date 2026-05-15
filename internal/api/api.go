package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fbauth "firebase.google.com/go/v4/auth"

	"github.com/dsionov/carwatch/internal/botcore"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/logstream"
	"github.com/dsionov/carwatch/internal/pricelist"
	"github.com/dsionov/carwatch/internal/storage"
)

type contextKey string

const (
	chatIDKey    contextKey = "chatID"
	emailKey     contextKey = "email"
	requestIDKey contextKey = "requestID"
)

type PollTrigger interface {
	TriggerPoll()
}

type Server struct {
	catalog          catalog.Catalog
	searches         storage.SearchStore
	listings         storage.ListingStore
	users            storage.UserStore
	linkTokens       storage.LinkTokenStore
	firebaseAuth     TokenVerifier
	prices           storage.PriceTracker
	admin            storage.AdminStore
	saved            storage.SavedListingStore
	hidden           storage.HiddenListingStore
	notifs           storage.NotificationStore
	logHub           *logstream.Hub
	logLevel         *slog.LevelVar
	poller           PollTrigger
	logger           *slog.Logger
	cfg              config.APIConfig
	botUsername      string
	startTime        time.Time
	rl               *rateLimiter
	ipRL             *ipRateLimiter
	vacuumMu         sync.Mutex
	fetchers         *fetcher.Factory
	priceListSvc     *pricelist.Service
	refreshMu        sync.Map
	lastRefreshSweep atomic.Int64 // unix nano of last sweep

	// Cumulative HTTP API metrics (since process start); see observeHTTPRequest.
	httpReqTotal   atomic.Uint64
	http2xx        atomic.Uint64
	http4xx        atomic.Uint64
	http5xx        atomic.Uint64
	httpDurationMs atomic.Uint64
}

func (s *Server) SetPollTrigger(p PollTrigger) {
	s.poller = p
}

func (s *Server) Shutdown() {
	if s.rl != nil {
		s.rl.stop()
	}
	if s.ipRL != nil {
		s.ipRL.stop()
	}
}

type Config struct {
	Catalog      catalog.Catalog
	Searches     storage.SearchStore
	Listings     storage.ListingStore
	Users        storage.UserStore
	LinkTokens   storage.LinkTokenStore
	Prices       storage.PriceTracker
	Admin        storage.AdminStore
	Saved        storage.SavedListingStore
	Hidden       storage.HiddenListingStore
	Notifs       storage.NotificationStore
	LogHub       *logstream.Hub
	LogLevel     *slog.LevelVar
	Logger       *slog.Logger
	API          config.APIConfig
	FirebaseAuth TokenVerifier
	BotUsername  string
	Fetchers     *fetcher.Factory
	PriceListSvc *pricelist.Service
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
		admin:        c.Admin,
		saved:        c.Saved,
		hidden:       c.Hidden,
		notifs:       c.Notifs,
		logHub:       c.LogHub,
		logLevel:     c.LogLevel,
		logger:       c.Logger,
		cfg:          c.API,
		botUsername:  c.BotUsername,
		startTime:    time.Now(),
		rl:           newRateLimiter(60, time.Second/60),
		ipRL:         newIPRateLimiter(20, time.Second/10, c.API.TrustForwardedFor),
		fetchers:     c.Fetchers,
		priceListSvc: c.PriceListSvc,
	}
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf [8]byte
		var id string
		if _, err := rand.Read(buf[:]); err != nil {
			id = strconv.FormatInt(time.Now().UnixNano(), 36)
		} else {
			id = hex.EncodeToString(buf[:])
		}

		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/catalog/manufacturers", s.listManufacturers)
	mux.HandleFunc("GET /api/v1/catalog/manufacturers/{id}/models", s.listModels)

	mux.HandleFunc("GET /api/v1/me", s.getMe)

	mux.HandleFunc("GET /api/v1/searches", s.listSearches)
	mux.HandleFunc("POST /api/v1/searches", s.createSearch)
	mux.HandleFunc("GET /api/v1/searches/{id}", s.getSearch)
	mux.HandleFunc("PUT /api/v1/searches/{id}", s.updateSearch)
	mux.HandleFunc("DELETE /api/v1/searches/{id}", s.deleteSearch)
	mux.HandleFunc("POST /api/v1/searches/{id}/pause", s.pauseSearch)
	mux.HandleFunc("POST /api/v1/searches/{id}/resume", s.resumeSearch)

	mux.HandleFunc("GET /api/v1/searches/{id}/listings", s.listListings)
	mux.HandleFunc("POST /api/v1/searches/{id}/refresh", s.refreshListings)
	mux.HandleFunc("GET /api/v1/listings/{token}", s.getListing)

	if s.admin != nil {
		mux.HandleFunc("GET /api/v1/admin/stats", s.requireAdmin(s.adminStats))
		mux.HandleFunc("GET /api/v1/admin/listings", s.requireAdmin(s.adminListListings))
		mux.HandleFunc("DELETE /api/v1/admin/listings/{token}", s.requireAdmin(s.adminDeleteListing))
		mux.HandleFunc("GET /api/v1/admin/searches", s.requireAdmin(s.adminListSearches))
		mux.HandleFunc("DELETE /api/v1/admin/searches/{id}", s.requireAdmin(s.adminDeleteSearch))
		mux.HandleFunc("GET /api/v1/admin/users", s.requireAdmin(s.adminListUsers))
		mux.HandleFunc("PATCH /api/v1/admin/users/{chatID}", s.requireAdmin(s.adminSetUserActive))
		mux.HandleFunc("DELETE /api/v1/admin/users/{chatID}", s.requireAdmin(s.adminDeleteUser))
		mux.HandleFunc("POST /api/v1/admin/purge", s.requireAdmin(s.adminPurgeTable))
		mux.HandleFunc("POST /api/v1/admin/vacuum", s.requireAdmin(s.adminVacuum))
		mux.HandleFunc("POST /api/v1/admin/sync-user-status", s.requireAdmin(s.adminSyncUserStatus))
		mux.HandleFunc("GET /api/v1/admin/price-history", s.requireAdmin(s.adminListPriceHistory))
		mux.HandleFunc("GET /api/v1/admin/seen-listings", s.requireAdmin(s.adminListSeenListings))
		mux.HandleFunc("GET /api/v1/admin/activity", s.requireAdmin(s.adminActivity))
		if s.logHub != nil {
			mux.HandleFunc("GET /api/v1/admin/logs", s.requireAdmin(s.adminLogs))
			mux.HandleFunc("GET /api/v1/admin/logs/stream", s.requireAdmin(s.adminLogStream))
		}
		if s.logLevel != nil {
			mux.HandleFunc("GET /api/v1/admin/logs/level", s.requireAdmin(s.adminGetLogLevel))
			mux.HandleFunc("PUT /api/v1/admin/logs/level", s.requireAdmin(s.adminSetLogLevel))
		}
	}

	if s.notifs != nil {
		mux.HandleFunc("GET /api/v1/notifications", s.listNotifications)
		mux.HandleFunc("GET /api/v1/notifications/count", s.notificationCount)
		mux.HandleFunc("POST /api/v1/notifications/seen", s.markNotificationsSeen)
		mux.HandleFunc("POST /api/v1/listings/{token}/seen", s.markListingSeen)
		mux.HandleFunc("DELETE /api/v1/listings/{token}/seen", s.unmarkListingSeen)
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

	chain := s.withMaxBody(mux)
	chain = s.withRateLimit(chain)
	chain = s.authMiddleware(chain)
	chain = s.withIPRateLimit(chain)
	chain = s.corsMiddleware(chain)
	chain = securityHeaders(chain)
	chain = s.withAccessLog(chain)
	return requestIDMiddleware(chain)
}

const maxRequestBody = 1 << 20 // 1 MB

func (s *Server) withMaxBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		}
		next.ServeHTTP(w, r)
	})
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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
				if bearer != s.cfg.AuthToken {
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

func chatIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(chatIDKey).(int64)
	if !ok || id <= 0 {
		return 0, false
	}
	return id, true
}

func emailFromContext(ctx context.Context) string {
	e, ok := ctx.Value(emailKey).(string)
	if !ok {
		return ""
	}
	return e
}

func requireChatID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := chatIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid user context")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("writeJSON failed", "error", err, "status", status)
	}
}

func (s *Server) handlerLogger(r *http.Request, extras ...any) *slog.Logger {
	fields := make([]any, 0, 6+len(extras))
	if reqID, ok := r.Context().Value(requestIDKey).(string); ok {
		fields = append(fields, "request_id", reqID)
	}
	if chatID, ok := chatIDFromContext(r.Context()); ok {
		fields = append(fields, "chat_id", chatID)
	}
	fields = append(fields, extras...)
	return s.logger.With(fields...)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseIntParam(w http.ResponseWriter, r *http.Request, name string, defaultVal int) (int, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return defaultVal, true
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s parameter: must be an integer", name))
		return 0, false
	}
	if v < 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s parameter: must be non-negative", name))
		return 0, false
	}
	return v, true
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
	return botcore.NormalizeKeywords(s)
}

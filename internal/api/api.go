package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fbauth "firebase.google.com/go/v4/auth"

	"github.com/dsionov/carwatch/internal/botcore"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/cwlog"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/logstream"
	"github.com/dsionov/carwatch/internal/pricelist"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/storage"
)

type contextKey string

const (
	chatIDKey    contextKey = "chatID"
	emailKey     contextKey = "email"
	requestIDKey contextKey = "requestID"
)

// maxConcurrentFetchesCeiling bounds api.max_concurrent_fetches so a
// misconfigured value cannot spawn an unbounded number of fetch goroutines.
const maxConcurrentFetchesCeiling = 64

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
	dedup            storage.DedupStore
	digests          storage.DigestStore
	alertPublisher   *broker.Publisher
	poller           PollTrigger
	logger           *slog.Logger
	cfg              config.APIConfig
	botUsername      string
	startTime        time.Time
	rl               *rateLimiter
	ipRL             *ipRateLimiter
	guestRL          *ipRateLimiter
	globalGuestRL    *globalBucket
	vacuumMu         sync.Mutex
	fetchers         *fetcher.Factory
	priceListSvc     *pricelist.Service
	pipeline         *scheduler.ListingPipeline
	pushSubs         storage.PushSubscriptionStore
	vapidPublicKey   string
	refreshMu        sync.Map
	lastRefreshSweep atomic.Int64 // unix nano of last sweep
	pollingInterval  time.Duration
	fetchSem         chan struct{}

	logHub     *logstream.Hub
	logLevel   *slog.LevelVar
	cycleLog   storage.CycleLogStore
	cycleStats storage.SearchCycleStatsStore
	activity   storage.SearchActivityStore
	extStatus  storage.ExtScanStatusStore
	removalBud *removalBudget
	vitals     *vitalsRing

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
	if s.guestRL != nil {
		s.guestRL.stop()
	}
	if s.alertPublisher != nil {
		if err := s.alertPublisher.Close(); err != nil && s.logger != nil {
			s.logger.Warn("failed to close alert publisher on shutdown", "error", err)
		}
	}
}

type Config struct {
	Catalog    catalog.Catalog
	Searches   storage.SearchStore
	Listings   storage.ListingStore
	Users      storage.UserStore
	LinkTokens storage.LinkTokenStore
	Prices     storage.PriceTracker
	Admin      storage.AdminStore
	Saved      storage.SavedListingStore
	Hidden     storage.HiddenListingStore
	Notifs     storage.NotificationStore
	PushSubs   storage.PushSubscriptionStore
	// Dedup, Digests and AlertPublisher wire the extension ingest path into the
	// same notification delivery the scheduler uses. All optional: nil disables
	// alerting from ingest (listings are still saved and shown in the web UI).
	Dedup            storage.DedupStore
	Digests          storage.DigestStore
	AlertPublisher   *broker.Publisher
	Logger           *slog.Logger
	API              config.APIConfig
	Push             config.PushConfig
	FirebaseAuth     TokenVerifier
	BotUsername      string
	Fetchers         *fetcher.Factory
	Yad2Fetcher      yad2.ItemFetcher
	EnricherConfig   config.EnricherConfig
	LogHub           *logstream.Hub
	LogLevel         *slog.LevelVar
	CycleLog         storage.CycleLogStore
	SearchCycleStats storage.SearchCycleStatsStore
	Activity         storage.SearchActivityStore
	// ExtScanStatus persists the extension's self-reported scan schedule so
	// /scheduler/status can serve the real "next scan" time. Optional: nil
	// falls back to estimating from the scheduler cycle log.
	ExtScanStatus   storage.ExtScanStatusStore
	PriceListSvc    *pricelist.Service
	PollingInterval time.Duration
	Bind            string
}

func New(c Config) *Server {
	if c.FirebaseAuth == nil && IsNonLocalBind(c.Bind) {
		// Reaching here on a non-local bind requires the explicit
		// api.allow_insecure_dev_auth opt-in (BuildAPI refuses otherwise).
		if c.Logger != nil {
			c.Logger.Warn("firebase auth not configured — using insecure dev auth mode on non-localhost bind address",
				"bind", c.Bind,
				"impact", "all API access is unauthenticated; for local development only")
		}
	}

	// Normalize the admin email once so isAdmin can compare exactly (no
	// per-request Unicode case folding).
	c.API.AdminEmail = strings.ToLower(strings.TrimSpace(c.API.AdminEmail))

	fetchCap := c.API.MaxConcurrentFetches
	if fetchCap <= 0 {
		fetchCap = 10
	}
	// Clamp to a sane ceiling so a misconfigured value cannot spawn an
	// unbounded number of concurrent fetch goroutines.
	if fetchCap > maxConcurrentFetchesCeiling {
		if c.Logger != nil {
			c.Logger.Warn("api.max_concurrent_fetches above ceiling, clamping",
				"configured", fetchCap, "ceiling", maxConcurrentFetchesCeiling)
		}
		fetchCap = maxConcurrentFetchesCeiling
	}

	pipeline := scheduler.NewListingPipeline(c.Listings, c.PriceListSvc, c.Logger)
	if c.Yad2Fetcher != nil {
		enricher := yad2.NewEnricher(c.Yad2Fetcher,
			c.Logger.With("component", "inline-enricher"),
			yad2.EnricherConfig{
				Delay:       c.EnricherConfig.BaseDelay,
				MaxPerCycle: c.EnricherConfig.InlineMaxPerCycle,
			})
		pipeline.SetInlineEnricher(enricher.Enrich)
	}

	return &Server{
		catalog:         c.Catalog,
		searches:        c.Searches,
		listings:        c.Listings,
		users:           c.Users,
		linkTokens:      c.LinkTokens,
		firebaseAuth:    c.FirebaseAuth,
		prices:          c.Prices,
		admin:           c.Admin,
		saved:           c.Saved,
		hidden:          c.Hidden,
		notifs:          c.Notifs,
		dedup:           c.Dedup,
		digests:         c.Digests,
		alertPublisher:  c.AlertPublisher,
		pushSubs:        c.PushSubs,
		vapidPublicKey:  c.Push.VAPIDPublicKey,
		logger:          c.Logger,
		cfg:             c.API,
		botUsername:     c.BotUsername,
		startTime:       time.Now(),
		rl:              newRateLimiter(60, time.Second/60),
		ipRL:            newIPRateLimiter(20, time.Second/10, c.API.TrustForwardedFor),
		guestRL:         newIPRateLimiter(15, 3*time.Minute, c.API.TrustForwardedFor),
		globalGuestRL:   newGlobalBucket(30, 10*time.Second),
		fetchers:        c.Fetchers,
		priceListSvc:    c.PriceListSvc,
		pipeline:        pipeline,
		logHub:          c.LogHub,
		logLevel:        c.LogLevel,
		cycleLog:        c.CycleLog,
		cycleStats:      c.SearchCycleStats,
		activity:        c.Activity,
		extStatus:       c.ExtScanStatus,
		pollingInterval: c.PollingInterval,
		vitals:          newVitalsRing(),
		removalBud:      newRemovalBudget(),
		fetchSem:        make(chan struct{}, fetchCap),
	}
}

func IsNonLocalBind(bind string) bool {
	b := strings.TrimSpace(bind)
	if b == "" {
		return false
	}
	host := b
	if h, _, err := net.SplitHostPort(b); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	return host != "127.0.0.1" && host != "localhost" && host != "::1"
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
		ctx = cwlog.WithRequestID(ctx, id)
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
	// --- Auth routes (existing, unchanged) ---
	authMux := http.NewServeMux()

	authMux.HandleFunc("GET /api/v1/catalog/manufacturers", s.listManufacturers)
	authMux.HandleFunc("GET /api/v1/catalog/manufacturers/{id}/models", s.listModels)
	authMux.HandleFunc("POST /api/v1/vitals", s.receiveVitals)

	authMux.HandleFunc("POST /api/v1/searches", s.createSearch)
	authMux.HandleFunc("GET /api/v1/searches/{id}", s.getSearch)
	authMux.HandleFunc("PUT /api/v1/searches/{id}", s.updateSearch)
	authMux.HandleFunc("DELETE /api/v1/searches/{id}", s.deleteSearch)
	authMux.HandleFunc("POST /api/v1/searches/{id}/pause", s.pauseSearch)
	authMux.HandleFunc("POST /api/v1/searches/{id}/resume", s.resumeSearch)

	authMux.HandleFunc("GET /api/v1/searches/{id}/stats", s.searchStats)
	authMux.HandleFunc("GET /api/v1/searches/{id}/listings", s.listListings)
	authMux.HandleFunc("POST /api/v1/searches/{id}/refresh", s.refreshListings)
	authMux.HandleFunc("GET /api/v1/listings/{token}", s.getListing)
	authMux.HandleFunc("GET /api/v1/listings/{token}/price-history", s.listingPriceHistory)
	authMux.HandleFunc("POST /api/v1/ext/ingest", s.ingestListings)

	if s.cycleStats != nil {
		authMux.HandleFunc("GET /api/v1/searches/cycle-stats", s.listSearchCycleStats)
	}

	if s.activity != nil {
		authMux.HandleFunc("GET /api/v1/searches/{id}/activity", s.searchActivity)
	}

	if s.admin != nil {
		authMux.HandleFunc("GET /api/v1/admin/stats", s.requireAdmin(s.adminStats))
		authMux.HandleFunc("GET /api/v1/admin/listings", s.requireAdmin(s.adminListListings))
		authMux.HandleFunc("DELETE /api/v1/admin/listings/{token}", s.requireAdmin(s.adminDeleteListing))
		authMux.HandleFunc("GET /api/v1/admin/searches", s.requireAdmin(s.adminListSearches))
		authMux.HandleFunc("DELETE /api/v1/admin/searches/{id}", s.requireAdmin(s.adminDeleteSearch))
		authMux.HandleFunc("GET /api/v1/admin/users", s.requireAdmin(s.adminListUsers))
		authMux.HandleFunc("PATCH /api/v1/admin/users/{chatID}", s.requireAdmin(s.adminSetUserActive))
		authMux.HandleFunc("DELETE /api/v1/admin/users/{chatID}", s.requireAdmin(s.adminDeleteUser))
		authMux.HandleFunc("POST /api/v1/admin/purge", s.requireAdmin(s.adminPurgeTable))
		authMux.HandleFunc("POST /api/v1/admin/reset-all", s.requireAdmin(s.adminResetAll))
		authMux.HandleFunc("POST /api/v1/admin/vacuum", s.requireAdmin(s.adminVacuum))
		authMux.HandleFunc("POST /api/v1/admin/sync-user-status", s.requireAdmin(s.adminSyncUserStatus))
		authMux.HandleFunc("GET /api/v1/admin/activity", s.requireAdmin(s.adminActivity))
		if s.cycleLog != nil {
			authMux.HandleFunc("GET /api/v1/admin/cycles", s.requireAdmin(s.adminCycles))
		}
		if s.logHub != nil {
			authMux.HandleFunc("GET /api/v1/admin/logs", s.requireAdmin(s.adminLogs))
			authMux.HandleFunc("GET /api/v1/admin/logs/stream", s.requireAdmin(s.adminLogStream))
		}
		if s.logLevel != nil {
			authMux.HandleFunc("GET /api/v1/admin/logs/level", s.requireAdmin(s.adminGetLogLevel))
			authMux.HandleFunc("PUT /api/v1/admin/logs/level", s.requireAdmin(s.adminSetLogLevel))
		}
	}

	if (s.cycleLog != nil && s.pollingInterval > 0) || s.extStatus != nil {
		authMux.HandleFunc("GET /api/v1/scheduler/status", s.schedulerStatus)
	}

	if s.notifs != nil {
		authMux.HandleFunc("GET /api/v1/notifications", s.listNotifications)
		authMux.HandleFunc("POST /api/v1/notifications/seen", s.markNotificationsSeen)
		authMux.HandleFunc("POST /api/v1/listings/{token}/seen", s.markListingSeen)
		authMux.HandleFunc("DELETE /api/v1/listings/{token}/seen", s.unmarkListingSeen)
	}

	if s.pushSubs != nil {
		authMux.HandleFunc("POST /api/v1/push/subscribe", s.pushSubscribe)
		authMux.HandleFunc("DELETE /api/v1/push/subscribe", s.pushUnsubscribe)
		authMux.HandleFunc("GET /api/v1/push/vapid-key", s.pushVAPIDKey)
	}

	authMux.HandleFunc("GET /api/v1/telegram/status", s.getTelegramStatus)
	if s.linkTokens != nil {
		authMux.HandleFunc("POST /api/v1/telegram/link", s.postTelegramLink)
	}

	if s.saved != nil && s.hidden != nil {
		authMux.HandleFunc("GET /api/v1/saved", s.listSaved)
		authMux.HandleFunc("POST /api/v1/listings/{token}/save", s.saveListing)
		authMux.HandleFunc("DELETE /api/v1/listings/{token}/save", s.unsaveListing)
		authMux.HandleFunc("POST /api/v1/listings/{token}/hide", s.hideListing)
		authMux.HandleFunc("DELETE /api/v1/listings/{token}/hide", s.unhideListing)
		authMux.HandleFunc("GET /api/v1/history", s.listHistory)
	}

	authChain := s.withMaxBody(authMux)
	authChain = s.withRateLimit(authChain)
	authChain = s.authMiddleware(authChain)

	// --- Optional-auth routes (read-only, return empty data for guests) ---
	optAuthMux := http.NewServeMux()
	optAuthMux.HandleFunc("GET /api/v1/me", s.getMe)
	optAuthMux.HandleFunc("GET /api/v1/searches", s.listSearches)
	if s.notifs != nil {
		optAuthMux.HandleFunc("GET /api/v1/notifications/count", s.notificationCount)
	}

	optAuthChain := s.withMaxBody(optAuthMux)
	optAuthChain = s.withRateLimit(optAuthChain)
	optAuthChain = s.optionalAuthMiddleware(optAuthChain)

	// --- Guest instant-search (rate-limited, no auth) ---
	guestMux := http.NewServeMux()
	guestMux.HandleFunc("POST /api/v1/guest/instant-search", s.instantSearch)

	guestChain := s.withMaxBody(guestMux)
	guestChain = s.withGuestRateLimit(guestChain)

	// --- Catalog routes (no auth, no guest rate limit) ---
	// Catalog data is static and cheap to serve; rate-limiting it penalises
	// the normal try-search flow where loading manufacturers + models already
	// consumes tokens before the user even searches.
	catalogMux := http.NewServeMux()
	catalogMux.HandleFunc("GET /api/v1/catalog/manufacturers", s.listManufacturers)
	catalogMux.HandleFunc("GET /api/v1/catalog/manufacturers/{id}/models", s.listModels)

	catalogChain := s.withMaxBody(catalogMux)

	// --- Capabilities (no auth) ---
	// Static, single-field, and needed before the UI renders, so it shares the
	// unrate-limited catalog treatment.
	capMux := http.NewServeMux()
	capMux.HandleFunc("GET /api/v1/capabilities", s.capabilities)
	capChain := s.withMaxBody(capMux)

	// --- Top-level router ---
	// More-specific prefixes are matched first by net/http.ServeMux.
	top := http.NewServeMux()
	top.Handle("/api/v1/guest/", guestChain)
	top.Handle("/api/v1/catalog/", catalogChain)
	top.Handle("GET /api/v1/capabilities", capChain)
	top.Handle("GET /api/v1/me", optAuthChain)
	top.Handle("GET /api/v1/searches", optAuthChain)
	if s.notifs != nil {
		top.Handle("GET /api/v1/notifications/count", optAuthChain)
	}
	top.Handle("/", authChain)

	// Shared outer middleware: requestID → accessLog → securityHeaders → CORS → ipRL
	chain := s.withIPRateLimit(top)
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

// optionalAuthMiddleware authenticates the request when a credential is
// offered, but allows anonymous callers through as guests (chatID 0) when none
// is. Handlers behind it call chatIDFromContext to tell guests from real users.
//
// "No credential" and "a credential that does not work" are NOT the same thing.
// A request that presents a Bearer token we cannot verify — expired, revoked,
// malformed — is a failed authentication and gets 401, not a guest response.
// Treating it as anonymous is how a stale token turns into silent data loss:
// GET /searches answered `200 []`, and the caller could not distinguish "you
// have no searches" from "your session expired". The browser extension, whose
// token expires roughly hourly, would then scan nothing while reporting
// success; a web client with a broken refresh would render an empty dashboard
// instead of sending the user to log in.
func (s *Server) optionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHdr := r.Header.Get("Authorization")
		bearer := bearerFromAuthHeader(authHdr)
		// An Authorization header we could not read a bearer token out of is
		// still an attempt to authenticate — do not silently downgrade it.
		credentialOffered := strings.TrimSpace(authHdr) != ""

		var chatID int64
		var userEmail string

		switch {
		case s.firebaseAuth != nil && bearer != "":
			tok, err := s.firebaseAuth.VerifyIDToken(r.Context(), bearer)
			if err != nil {
				writeAuthError(w, "invalid or expired token")
				return
			}
			userEmail = emailFromClaims(tok)
			id, upsertErr := s.users.UpsertWebUser(r.Context(), tok.UID, userEmail)
			if upsertErr != nil {
				s.logger.Error("upsert web user", "error", upsertErr)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			chatID = id

		case s.firebaseAuth != nil && credentialOffered:
			// Authorization present but not a usable Bearer token.
			writeAuthError(w, "invalid or expired token")
			return

		case s.firebaseAuth == nil:
			// Dev auth. A configured dev token must still be presented correctly
			// when one is offered at all.
			if s.cfg.AuthToken != "" {
				if bearer == s.cfg.AuthToken {
					chatID = s.cfg.DevChatID
				} else if credentialOffered {
					writeAuthError(w, "invalid or expired token")
					return
				}
			} else {
				chatID = s.cfg.DevChatID
			}
		}

		ctx := context.WithValue(r.Context(), chatIDKey, chatID)
		ctx = context.WithValue(ctx, emailKey, userEmail)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeAuthError rejects a failed authentication. The WWW-Authenticate header
// tells clients (and the extension) that the credential — not the request — is
// what went wrong, so they re-authenticate instead of retrying blindly.
func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="carwatch", error="invalid_token"`)
	writeError(w, http.StatusUnauthorized, msg)
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
				writeAuthError(w, "invalid or missing token")
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
			writeAuthError(w, "invalid or missing token")
			return
		} else if s.firebaseAuth == nil {
			if s.cfg.AuthToken != "" {
				if bearer != s.cfg.AuthToken {
					writeAuthError(w, "invalid or missing token")
					return
				}
			}
			chatID = s.cfg.DevChatID
			if chatID == 0 {
				writeError(w, http.StatusUnauthorized, "no user configured")
				return
			}
		} else {
			writeAuthError(w, "invalid or missing token")
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

func (s *Server) resolveCanonicalChatID(ctx context.Context, webChatID int64) int64 {
	if s.users == nil {
		return webChatID
	}
	linked, err := s.users.GetLinkedTelegramUser(ctx, webChatID)
	if err != nil {
		s.logger.Warn("resolve canonical chatID: lookup failed", "web_chat_id", webChatID, "error", err)
		return webChatID
	}
	if linked == nil {
		return webChatID
	}
	return linked.ChatID
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

func (s *Server) requireResolvedChatID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := requireChatID(w, r)
	if !ok {
		return 0, false
	}
	return s.resolveCanonicalChatID(r.Context(), id), true
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

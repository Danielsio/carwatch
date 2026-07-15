// Package app provides shared initialization helpers used by all entry points
// (cmd/api-server, cmd/bot-poller, cmd/scraper, cmd/notifier). Each binary's main.go
// is thin wiring that composes these building blocks.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/dsionov/carwatch/internal/api"
	cwbot "github.com/dsionov/carwatch/internal/bot"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/logstream"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/notifier/telegram"
	"github.com/dsionov/carwatch/internal/notifier/webpush"
	"github.com/dsionov/carwatch/internal/pricelist"
	"github.com/dsionov/carwatch/internal/spa"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/postgres"
	"github.com/dsionov/carwatch/internal/telemetry"
	"github.com/dsionov/carwatch/web"
)

// LoadConfig reads and validates the configuration file at the given path.
func LoadConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

// OpenStore opens a PostgreSQL store using the configuration's DSN and
// runs pending migrations. Pass skipMigrate=true to connect without
// running migrations (useful when a separate deploy step handles them).
func OpenStore(cfg *config.Config, skipMigrate ...bool) (*postgres.Store, error) {
	migrationsPath := cfg.Storage.MigrationsPath
	if len(skipMigrate) > 0 && skipMigrate[0] {
		migrationsPath = ""
	}
	return postgres.New(cfg.Storage.DSN, migrationsPath)
}

// Yad2Source abstracts the Yad2 fetcher so both the HTTP-based Yad2Fetcher
// and the Chrome-based RodFetcher can be used interchangeably.
type Yad2Source interface {
	fetcher.Fetcher
	FetchItem(ctx context.Context, token string) (yad2.ItemDetails, error)
	FetchRawPage(ctx context.Context, url string) ([]byte, error)
	Close()
}

// FetcherBundle groups all fetcher-related objects returned by BuildFetchers.
type FetcherBundle struct {
	Yad2     Yad2Source
	Caching  fetcher.Fetcher // full chain: CircuitBreaker → Cache → Paginator → Yad2
	Targeted fetcher.Fetcher // without circuit breaker: Cache → Paginator → Yad2
	Factory  *fetcher.Factory
	Pool     *fetcher.ProxyPool
}

// BuildFetchers creates the Yad2 fetcher, paginating/caching wrappers,
// circuit breaker, and fetcher factory.
func BuildFetchers(cfg *config.Config, logger *slog.Logger) (*FetcherBundle, error) {
	var proxyPool *fetcher.ProxyPool
	if len(cfg.HTTP.Proxies) > 0 {
		proxyPool = fetcher.NewProxyPool(cfg.HTTP.Proxies)
	}

	yad2Logger := logger.With("component", "yad2")
	var yad2Source Yad2Source
	if cfg.HTTP.ChromeBin != "" {
		rodFetcher, err := yad2.NewRodFetcher(cfg.HTTP.ChromeBin, yad2Logger)
		if err != nil {
			return nil, fmt.Errorf("create rod fetcher: %w", err)
		}
		yad2Source = rodFetcher
		logger.Info("using chrome browser fetcher", "bin", cfg.HTTP.ChromeBin)
	} else if cfg.HTTP.RelayURL != "" {
		feedClient := yad2.NewRelayClient(cfg.HTTP.RelayURL, cfg.HTTP.RelaySecret, cfg.HTTP.UserAgents)
		itemClient := yad2.NewRelayClient(cfg.HTTP.RelayURL, cfg.HTTP.RelaySecret, cfg.HTTP.UserAgents)
		yad2Fetcher := yad2.NewFetcherWithClients(feedClient, itemClient, cfg.HTTP.UserAgents, yad2Logger)
		yad2Fetcher.SetUseGwFeed(cfg.HTTP.UseGwFeed)
		yad2Source = yad2Fetcher
		logger.Info("using cloudflare relay", "relay_url", cfg.HTTP.RelayURL)
	} else {
		var yad2Fetcher *yad2.Yad2Fetcher
		var err error
		if proxyPool != nil {
			yad2Fetcher, err = yad2.NewFetcherWithProxyPool(cfg.HTTP.UserAgents, proxyPool, yad2Logger)
		} else {
			yad2Fetcher, err = yad2.NewFetcher(cfg.HTTP.UserAgents, cfg.HTTP.Proxy, yad2Logger)
		}
		if err != nil {
			return nil, fmt.Errorf("create fetcher: %w", err)
		}
		yad2Fetcher.SetUseGwFeed(cfg.HTTP.UseGwFeed)
		yad2Source = yad2Fetcher
	}

	paginatingFetcher := fetcher.NewPaginatingFetcher(yad2Source, cfg.HTTP.MaxPages)
	cachingFetcher := fetcher.NewCachingFetcher(paginatingFetcher, 5*time.Minute)
	yad2CB := fetcher.NewCircuitBreaker(cachingFetcher, 5, 10*time.Minute,
		fetcher.WithCBLogger(logger.With("component", "circuit_breaker", "source", "yad2")))

	fetcherFactory := fetcher.NewFactory()
	fetcherFactory.Register("yad2", yad2CB)

	return &FetcherBundle{
		Yad2:     yad2Source,
		Caching:  yad2CB,
		Targeted: cachingFetcher,
		Factory:  fetcherFactory,
		Pool:     proxyPool,
	}, nil
}

// BotBundle groups the bot handler, Telegram notifier, and multi-notifier
// returned by BuildBot.
type BotBundle struct {
	Handler    *cwbot.Bot
	TgNotifier *telegram.Notifier
	Multi      *notifier.MultiNotifier
}

// BuildBot creates the Telegram bot handler, Telegram notifier, and
// multi-notifier (Telegram + optional WebPush).
func BuildBot(cfg *config.Config, store *postgres.Store, dynCatalog *catalog.DynamicCatalog, h *health.Status, logger *slog.Logger) (*BotBundle, error) {
	botHandler := cwbot.New(nil, store, store, cwbot.Config{
		AdminChatID:            cfg.Telegram.AdminChatID,
		MaxSearches:            cfg.Telegram.MaxSearches,
		BotUsername:            cfg.Telegram.BotUsername,
		PollInterval:           cfg.Polling.Interval,
		QuickStartManufacturer: cfg.Telegram.QuickStartManufacturer,
		QuickStartModel:        cfg.Telegram.QuickStartModel,
		Health:                 h,
		Digests:                store,
		Listings:               store,
		Saved:                  store,
		Hidden:                 store,
		DailyDigests:           store,
		Catalog:                dynCatalog,
		LinkTokens:             store,
	}, logger.With("component", "bot"))

	tgNotif, err := telegram.New(cfg.Telegram.Token, logger.With("component", "telegram"),
		tgbot.WithDefaultHandler(botHandler.DefaultHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	botHandler.SetBot(tgNotif.Bot())
	botHandler.RegisterHandlers()

	multi := notifier.NewMultiNotifier(store, logger.With("component", "notifier"))
	if err := multi.Register("telegram", tgNotif); err != nil {
		return nil, fmt.Errorf("register telegram notifier: %w", err)
	}

	if cfg.Push.VAPIDPublicKey != "" && cfg.Push.VAPIDPrivateKey != "" {
		wpNotif := webpush.New(store, cfg.Push.VAPIDPublicKey, cfg.Push.VAPIDPrivateKey, cfg.Push.VAPIDSubject, logger.With("component", "webpush"))
		if err := multi.Register("webpush", wpNotif); err != nil {
			return nil, fmt.Errorf("register webpush notifier: %w", err)
		}
		logger.Info("webpush notifier registered")
	}

	return &BotBundle{
		Handler:    botHandler,
		TgNotifier: tgNotif,
		Multi:      multi,
	}, nil
}

// BuildMultiNotifier creates a multi-notifier with Telegram and optional
// WebPush channels, without creating a bot handler. Used by the notifier
// worker which only needs to deliver messages.
func BuildMultiNotifier(cfg *config.Config, userStore storage.UserStore, subStore webpush.SubscriptionStore, logger *slog.Logger) (*notifier.MultiNotifier, error) {
	tgNotif, err := telegram.New(cfg.Telegram.Token, logger.With("component", "telegram"))
	if err != nil {
		return nil, fmt.Errorf("create telegram notifier: %w", err)
	}

	multi := notifier.NewMultiNotifier(userStore, logger.With("component", "notifier"))
	if err := multi.Register("telegram", tgNotif); err != nil {
		return nil, fmt.Errorf("register telegram notifier: %w", err)
	}

	if cfg.Push.VAPIDPublicKey != "" && cfg.Push.VAPIDPrivateKey != "" {
		wpNotif := webpush.New(subStore, cfg.Push.VAPIDPublicKey, cfg.Push.VAPIDPrivateKey, cfg.Push.VAPIDSubject, logger.With("component", "webpush"))
		if err := multi.Register("webpush", wpNotif); err != nil {
			return nil, fmt.Errorf("register webpush notifier: %w", err)
		}
		logger.Info("webpush notifier registered")
	}

	return multi, nil
}

// BuildDynamicCatalog creates and loads the dynamic catalog from the Yad2
// fetcher.
func BuildDynamicCatalog(ctx context.Context, yad2Source Yad2Source, logger *slog.Logger) *catalog.DynamicCatalog {
	dynCatalog := catalog.NewDynamic(logger)
	pageFetcher := &catalog.HTTPPageFetcher{
		GetPage: yad2Source.FetchRawPage,
	}
	dynCatalog.Load(ctx, pageFetcher)
	logger.Info("dynamic catalog loaded")
	return dynCatalog
}

// BuildAPI creates the API server with all REST endpoints.
func BuildAPI(cfg *config.Config, store *postgres.Store, dynCatalog *catalog.DynamicCatalog, logger *slog.Logger, fetcherFactory *fetcher.Factory, yad2Source Yad2Source, plSvc *pricelist.Service, logHub *logstream.Hub, logLevel *slog.LevelVar) (*api.Server, error) {
	if api.IsNonLocalBind(cfg.HTTP.Bind) && cfg.Telemetry.AuthToken == "" {
		return nil, fmt.Errorf("telemetry.auth_token must be configured for non-local bind address %q", cfg.HTTP.Bind)
	}

	var firebaseAuth api.TokenVerifier
	if cfg.Firebase.ProjectID != "" {
		v, err := api.NewFirebaseVerifier(cfg.Firebase.CredentialsFile, cfg.Firebase.CredentialsJSON, cfg.Firebase.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("init firebase: %w", err)
		}
		firebaseAuth = v
	}
	if firebaseAuth == nil && api.IsNonLocalBind(cfg.HTTP.Bind) {
		if !cfg.API.AllowInsecureDevAuth {
			return nil, fmt.Errorf("firebase auth must be configured for non-local bind address %q "+
				"(set api.allow_insecure_dev_auth: true to override for local development only)", cfg.HTTP.Bind)
		}
		logger.Warn("SECURITY: starting API with unauthenticated dev-auth on a non-local bind address",
			"bind", cfg.HTTP.Bind,
			"reason", "api.allow_insecure_dev_auth is enabled",
			"impact", "anyone who can reach this address has full API access — never enable in production")
	}

	cfg.API.AdminChatID = cfg.Telegram.AdminChatID
	cfg.API.MaxSearches = cfg.Telegram.MaxSearches

	// Alert publisher lets the extension ingest path emit notifications on the
	// same Redis stream the scheduler uses (the notifier worker consumes it).
	// Best-effort: if Redis is unavailable the API still serves; ingest just
	// won't send alerts.
	var alertPublisher *broker.Publisher
	if cfg.Redis.Addr != "" {
		p, err := broker.NewPublisher(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		if err != nil {
			logger.Warn("alert publisher unavailable; extension ingest will not send notifications",
				"error", err)
		} else {
			alertPublisher = p
		}
	}

	apiServer := api.New(api.Config{
		Catalog:          dynCatalog,
		Searches:         store,
		Listings:         store,
		Users:            store,
		LinkTokens:       store,
		Prices:           store,
		Admin:            store,
		Saved:            store,
		Hidden:           store,
		Notifs:           store,
		PushSubs:         store,
		Dedup:            store,
		Digests:          store,
		AlertPublisher:   alertPublisher,
		Logger:           logger,
		API:              cfg.API,
		Push:             cfg.Push,
		FirebaseAuth:     firebaseAuth,
		BotUsername:      cfg.Telegram.BotUsername,
		LogHub:           logHub,
		LogLevel:         logLevel,
		CycleLog:         store,
		SearchCycleStats: store,
		Activity:         store,
		ExtScanStatus:    store,
		ExtTokens:        store,
		PollingInterval:  cfg.Polling.Interval,
		Fetchers:         fetcherFactory,
		Yad2Fetcher:      yad2Source,
		EnricherConfig:   cfg.Enricher,
		PriceListSvc:     plSvc,
		Bind:             cfg.HTTP.Bind,
	})

	return apiServer, nil
}

// BuildPriceListService creates the pricelist HTTP client and service.
func BuildPriceListService(cfg *config.Config, store *postgres.Store, logger *slog.Logger) (*pricelist.Service, func(), error) {
	var plClient yad2.HTTPDoer
	if cfg.HTTP.RelayURL != "" {
		plClient = yad2.NewRelayClient(cfg.HTTP.RelayURL, cfg.HTTP.RelaySecret, cfg.HTTP.UserAgents)
	} else {
		c, err := yad2.NewClient(cfg.HTTP.UserAgents, cfg.HTTP.Proxy)
		if err != nil {
			return nil, nil, fmt.Errorf("create pricelist client: %w", err)
		}
		plClient = c
	}
	plHTTP := pricelist.NewYad2Client(plClient)
	svc := pricelist.NewService(store, plHTTP, logger.With("component", "api-pricelist"))
	cleanup := func() { plClient.Close() }
	return svc, cleanup, nil
}

// serve starts srv in the background and reports a listener failure on the
// returned channel. A failed listener is fatal, not a warning: the process it
// belongs to keeps running with nothing served — Docker sees an *unhealthy*
// container, and `restart: unless-stopped` does not restart unhealthy
// containers, so the zombie survives until a human notices. Callers must turn
// this into a non-zero exit (see GuardListeners).
//
// The channel is buffered so the goroutine never blocks if nobody is listening,
// and closed on a clean Shutdown so a receiver sees a nil error, not a hang.
func serve(srv *http.Server, logger *slog.Logger, what string) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(what+" failed", "addr", srv.Addr, "error", err)
			errCh <- fmt.Errorf("%s (%s): %w", what, srv.Addr, err)
		}
	}()
	return errCh
}

// BuildHTTPServer creates and starts the HTTP server with health, API, SPA,
// and metrics endpoints. The returned channel reports a fatal listener failure.
func BuildHTTPServer(cfg *config.Config, h *health.Status, apiServer *api.Server, logger *slog.Logger) (*http.Server, <-chan error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.PublicHandler())
	mux.Handle("/api/v1/", apiServer.Routes())
	mux.Handle("/", spa.Handler(web.DistFS()))

	metricsHandler, err := telemetry.MetricsHandler()
	if err != nil {
		logger.Error("metrics handler setup failed", "error", err)
	} else {
		mux.Handle(cfg.Telemetry.MetricsPath, metricsAuthMiddleware(cfg.HTTP.Bind, cfg.Telemetry.AuthToken, metricsHandler))
	}

	srv := &http.Server{
		Addr:              cfg.HTTP.Bind,
		Handler:           otelhttp.NewHandler(mux, "carwatch"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv, serve(srv, logger, "http server")
}

// BuildHealthServer creates and starts a minimal HTTP server with only the
// health endpoint, for use by headless services (scraper, notifier). The
// returned channel reports a fatal listener failure.
func BuildHealthServer(bind string, h *health.Status, logger *slog.Logger) (*http.Server, <-chan error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.PublicHandler())

	srv := &http.Server{
		Addr:              bind,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return srv, serve(srv, logger, "health server")
}

// ListenGuard ties a service's lifetime to its listeners: when one of them
// fails to serve, the guarded context is cancelled — unwinding whatever
// blocking loop the service is running (scheduler, Redis consumer, bot poller)
// — and Wrap turns the resulting shutdown into the listener's error, so the
// process exits non-zero and Docker's restart policy takes over.
type ListenGuard struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu  sync.Mutex
	err error
}

// GuardListeners derives a context from parent that is cancelled as soon as any
// of errChs reports a failure. Callers must defer Stop.
func GuardListeners(parent context.Context, errChs ...<-chan error) *ListenGuard {
	ctx, cancel := context.WithCancel(parent)
	g := &ListenGuard{ctx: ctx, cancel: cancel}
	for _, ch := range errChs {
		go func(ch <-chan error) {
			select {
			case err, ok := <-ch:
				if !ok || err == nil {
					return // clean shutdown closed the channel
				}
				g.mu.Lock()
				if g.err == nil {
					g.err = err
				}
				g.mu.Unlock()
				cancel()
			case <-ctx.Done():
			}
		}(ch)
	}
	return g
}

// Context returns the context services should run under.
func (g *ListenGuard) Context() context.Context { return g.ctx }

// Stop releases the guard's resources.
func (g *ListenGuard) Stop() { g.cancel() }

// Err reports the listener failure that aborted the service, if any.
func (g *ListenGuard) Err() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.err
}

// Wrap resolves a service's exit into the error the process should die with.
//
// A listener failure wins: whatever the service's blocking loop returned is
// just the echo of the cancellation we triggered. Otherwise a context
// cancellation means a signal-driven shutdown — that is a *clean* exit, not a
// fatal error, so it resolves to nil (previously every SIGTERM logged "fatal"
// and exited 1, making a graceful stop indistinguishable from a crash).
func (g *ListenGuard) Wrap(runErr error) error {
	if err := g.Err(); err != nil {
		return err
	}
	if errors.Is(runErr, context.Canceled) {
		return nil
	}
	return runErr
}

// NewLogHandler creates a slog.Handler, choosing between pretty (tint) and
// JSON output based on the format string and terminal detection.
func NewLogHandler(format string, level slog.Leveler) slog.Handler {
	fd := os.Stdout.Fd()
	usePretty := format == "pretty" ||
		(format == "auto" && (isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)))

	if usePretty {
		return tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
		})
	}
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
}

// InitTelemetry initializes OpenTelemetry tracing and metrics. Returns a
// shutdown function that should be deferred.
func InitTelemetry(ctx context.Context, serviceName, version string, cfg *config.Config, logger *slog.Logger) (func(context.Context) error, error) {
	shutdown, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:    serviceName,
		ServiceVersion: version,
		Exporter:       cfg.Telemetry.TracesExporter,
		OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
	})
	if err != nil {
		return nil, fmt.Errorf("init telemetry: %w", err)
	}

	if err := telemetry.InitMetrics(); err != nil {
		logger.Error("init metrics failed", "error", err)
	}

	return shutdown, nil
}

// SetupLogger creates a slog.Logger from the configuration, applying log
// level and format. Returns the logger and the underlying LevelVar (so
// callers can adjust the level at runtime if needed).
func SetupLogger(cfg *config.Config) (*slog.Logger, *slog.LevelVar, error) {
	logLevel, err := config.ParseLogLevel(cfg.LogLevel)
	if err != nil {
		return nil, nil, fmt.Errorf("parse log_level %q: %w", cfg.LogLevel, err)
	}
	var logLevelVar slog.LevelVar
	logLevelVar.Set(logLevel)
	handler := NewLogHandler(cfg.LogFormat, &logLevelVar)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, &logLevelVar, nil
}

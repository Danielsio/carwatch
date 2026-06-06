// Package app provides shared initialization helpers used by all entry points
// (cmd/api-server, cmd/bot-poller, cmd/scraper, cmd/notifier). Each binary's main.go
// is thin wiring that composes these building blocks.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/dsionov/carwatch/internal/api"
	cwbot "github.com/dsionov/carwatch/internal/bot"
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

// FetcherBundle groups all fetcher-related objects returned by BuildFetchers.
type FetcherBundle struct {
	Yad2     *yad2.Yad2Fetcher
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

	paginatingFetcher := fetcher.NewPaginatingFetcher(yad2Fetcher, cfg.HTTP.MaxPages)
	cachingFetcher := fetcher.NewCachingFetcher(paginatingFetcher, 5*time.Minute)
	yad2CB := fetcher.NewCircuitBreaker(cachingFetcher, 5, 10*time.Minute,
		fetcher.WithCBLogger(logger.With("component", "circuit_breaker", "source", "yad2")))

	fetcherFactory := fetcher.NewFactory()
	fetcherFactory.Register("yad2", yad2CB)

	return &FetcherBundle{
		Yad2:     yad2Fetcher,
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
func BuildDynamicCatalog(ctx context.Context, yad2Fetcher *yad2.Yad2Fetcher, logger *slog.Logger) *catalog.DynamicCatalog {
	dynCatalog := catalog.NewDynamic(logger)
	pageFetcher := &catalog.HTTPPageFetcher{
		GetPage: yad2Fetcher.FetchRawPage,
	}
	dynCatalog.Load(ctx, pageFetcher)
	logger.Info("dynamic catalog loaded")
	return dynCatalog
}

// BuildAPI creates the API server with all REST endpoints.
func BuildAPI(cfg *config.Config, store *postgres.Store, dynCatalog *catalog.DynamicCatalog, logger *slog.Logger, fetcherFactory *fetcher.Factory, plSvc *pricelist.Service, logHub *logstream.Hub, logLevel *slog.LevelVar) (*api.Server, error) {
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
		return nil, fmt.Errorf("firebase auth must be configured for non-local bind address %q", cfg.HTTP.Bind)
	}

	cfg.API.AdminChatID = cfg.Telegram.AdminChatID
	cfg.API.MaxSearches = cfg.Telegram.MaxSearches
	apiServer := api.New(api.Config{
		Catalog:      dynCatalog,
		Searches:     store,
		Listings:     store,
		Users:        store,
		LinkTokens:   store,
		Prices:       store,
		Admin:        store,
		Saved:        store,
		Hidden:       store,
		Notifs:       store,
		PushSubs:     store,
		Logger:       logger,
		API:          cfg.API,
		Push:         cfg.Push,
		FirebaseAuth: firebaseAuth,
		BotUsername:  cfg.Telegram.BotUsername,
		LogHub:       logHub,
		LogLevel:     logLevel,
		CycleLog:     store,
		Fetchers:     fetcherFactory,
		PriceListSvc: plSvc,
		Bind:         cfg.HTTP.Bind,
	})

	return apiServer, nil
}

// BuildPriceListService creates the pricelist HTTP client and service.
func BuildPriceListService(cfg *config.Config, store *postgres.Store, logger *slog.Logger) (*pricelist.Service, func(), error) {
	plClient, err := yad2.NewClient(cfg.HTTP.UserAgents, cfg.HTTP.Proxy)
	if err != nil {
		return nil, nil, fmt.Errorf("create pricelist client: %w", err)
	}
	plHTTP := pricelist.NewYad2Client(plClient)
	svc := pricelist.NewService(store, plHTTP, logger.With("component", "api-pricelist"))
	cleanup := func() { plClient.Close() }
	return svc, cleanup, nil
}

// BuildHTTPServer creates and starts the HTTP server with health, API, SPA,
// and metrics endpoints.
func BuildHTTPServer(cfg *config.Config, h *health.Status, apiServer *api.Server, logger *slog.Logger) *http.Server {
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
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
		}
	}()
	return srv
}

// BuildHealthServer creates and starts a minimal HTTP server with only the
// health endpoint, for use by headless services (scraper, notifier).
func BuildHealthServer(bind string, h *health.Status, logger *slog.Logger) *http.Server {
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
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server failed", "error", err)
		}
	}()
	return srv
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

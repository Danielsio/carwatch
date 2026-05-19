package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"

	"github.com/dsionov/carwatch/internal/api"
	cwbot "github.com/dsionov/carwatch/internal/bot"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/fetcher/winwin"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/logstream"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/notifier/telegram"
	"github.com/dsionov/carwatch/internal/notifier/webpush"
	"github.com/dsionov/carwatch/internal/pricelist"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/spa"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/storage/postgres"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
	"github.com/dsionov/carwatch/web"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("carwatch %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
		return
	}

	logger := slog.New(newLogHandler("auto", slog.LevelInfo))

	if err := run(*configPath, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, logger *slog.Logger) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logLevel, err := config.ParseLogLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("parse log_level %q: %w", cfg.LogLevel, err)
	}
	var logLevelVar slog.LevelVar
	logLevelVar.Set(logLevel)
	logHub := logstream.NewHub(2000)
	baseHandler := newLogHandler(cfg.LogFormat, &logLevelVar)
	teeHandler := logstream.NewTeeHandler(baseHandler, logHub,
		"yad2", "winwin", "scheduler", "enricher",
		"api-pricelist", "bot", "telegram", "notifier",
		"circuit_breaker",
	)
	logger = slog.New(teeHandler)
	slog.SetDefault(logger)
	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	store, err := openStore(cfg)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer func() { _ = store.Close() }()

	yad2Fetcher, cachingFetcher, fetcherFactory, _, err := buildFetchers(cfg, logger)
	if err != nil {
		return err
	}

	dynCatalog := catalog.NewDynamic(logger)
	pageFetcher := &catalog.HTTPPageFetcher{
		GetPage: yad2Fetcher.FetchRawPage,
	}
	dynCatalog.Load(context.Background(), pageFetcher)
	logger.Info("dynamic catalog loaded")

	h := health.New()
	h.SetVersion(version)
	h.SetUserCounter(store)
	h.SetSearchCounter(store)
	h.SetDBSizer(store)

	botHandler, tgNotif, multi, err := buildBot(cfg, store, dynCatalog, h, logger)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := multi.Connect(ctx); err != nil {
		return fmt.Errorf("connect notifiers: %w", err)
	}
	defer func() { _ = multi.Disconnect() }()

	plClient, err := yad2.NewClient(cfg.HTTP.UserAgents, cfg.HTTP.Proxy)
	if err != nil {
		return fmt.Errorf("create pricelist client: %w", err)
	}
	defer plClient.Close()
	plHTTP := pricelist.NewYad2Client(plClient)

	apiPriceListSvc := pricelist.NewService(store, plHTTP, logger.With("component", "api-pricelist"))
	apiServer, err := buildAPI(cfg, store, dynCatalog, logHub, &logLevelVar, logger, fetcherFactory, apiPriceListSvc)
	if err != nil {
		return err
	}
	defer apiServer.Shutdown()

	srv := buildHTTPServer(cfg, h, apiServer, logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("http server shutdown failed", "error", err)
		}
	}()

	kmEnricher := yad2.NewEnricher(yad2Fetcher, logger.With("component", "enricher"), yad2.EnricherConfig{})

	sched, err := scheduler.NewWithOptions(cfg, cachingFetcher, store, multi, logger.With("component", "scheduler"), scheduler.Options{
		Observer:         h,
		Queue:            store,
		Prices:           store,
		ConfigPath:       configPath,
		FetcherFactory:   fetcherFactory,
		ListingStore:     store,
		SearchStore:      store,
		UserStore:        store,
		DigestStore:      store,
		HiddenStore:      store,
		CatalogIngester:  dynCatalog,
		CarNames:         dynCatalog,
		KmEnricher:       kmEnricher,
		MarketStore:      store,
		PriceListStore:   store,
		PriceListSvc:     apiPriceListSvc,
		DailyDigestStore: store,
	})
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	botHandler.SetPollTrigger(sched)
	apiServer.SetPollTrigger(sched)
	botHandler.StartCleanup(ctx)

	go func() {
		const maxBackoff = 30 * time.Second
		backoff := time.Second
		for {
			h.MarkBotPollingAlive()
			logger.Info("telegram bot polling loop starting")
			tgNotif.Bot().Start(ctx)
			if ctx.Err() != nil {
				return
			}
			logger.Error("telegram bot polling loop exited unexpectedly, restarting", "backoff", backoff.String())
			h.MarkBotPollingDead()
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
		}
	}()
	// health marked inside goroutine
	logger.Info("bot started",
		"health", "http://"+cfg.HTTP.Bind+"/healthz",
	)

	h.MarkSchedulerStarted()
	return sched.Run(ctx)
}

func buildFetchers(cfg *config.Config, logger *slog.Logger) (*yad2.Yad2Fetcher, fetcher.Fetcher, *fetcher.Factory, *fetcher.ProxyPool, error) {
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
		return nil, nil, nil, nil, fmt.Errorf("create fetcher: %w", err)
	}

	paginatingFetcher := fetcher.NewPaginatingFetcher(yad2Fetcher, cfg.HTTP.MaxPages)
	cachingFetcher := fetcher.NewCachingFetcher(paginatingFetcher, 5*time.Minute)
	yad2CB := fetcher.NewCircuitBreaker(cachingFetcher, 5, 10*time.Minute,
		fetcher.WithCBLogger(logger.With("component", "circuit_breaker", "source", "yad2")))

	winwinLogger := logger.With("component", "winwin")
	var winwinFetcher *winwin.WinWinFetcher
	if proxyPool != nil {
		winwinFetcher, err = winwin.NewFetcherWithProxyPool(cfg.HTTP.UserAgents, proxyPool, winwinLogger)
	} else {
		winwinFetcher, err = winwin.NewFetcher(cfg.HTTP.UserAgents, cfg.HTTP.Proxy, winwinLogger)
	}
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create winwin fetcher: %w", err)
	}
	cachingWinwin := fetcher.NewCachingFetcher(winwinFetcher, 5*time.Minute)
	winwinCB := fetcher.NewCircuitBreaker(cachingWinwin, 5, 10*time.Minute,
		fetcher.WithCBLogger(logger.With("component", "circuit_breaker", "source", "winwin")))

	fetcherFactory := fetcher.NewFactory()
	fetcherFactory.Register("yad2", yad2CB)
	fetcherFactory.Register("winwin", winwinCB)

	return yad2Fetcher, cachingFetcher, fetcherFactory, proxyPool, nil
}

func openStore(cfg *config.Config) (storage.Store, error) {
	switch cfg.Storage.Driver {
	case "postgres":
		return postgres.New(cfg.Storage.DSN, cfg.Storage.MigrationsPath)
	default:
		return sqlite.New(cfg.Storage.DBPath)
	}
}

func buildBot(cfg *config.Config, store storage.Store, dynCatalog *catalog.DynamicCatalog, h *health.Status, logger *slog.Logger) (*cwbot.Bot, *telegram.Notifier, *notifier.MultiNotifier, error) {
	botHandler := cwbot.New(nil, store, store, cwbot.Config{
		AdminChatID:  cfg.Telegram.AdminChatID,
		MaxSearches:  cfg.Telegram.MaxSearches,
		BotUsername:  cfg.Telegram.BotUsername,
		PollInterval: cfg.Polling.Interval,
		Health:       h,
		Digests:      store,
		Listings:     store,
		Saved:        store,
		Hidden:       store,
		DailyDigests: store,
		Catalog:      dynCatalog,
		LinkTokens:   store,
	}, logger.With("component", "bot"))

	tgNotif, err := telegram.New(cfg.Telegram.Token, logger.With("component", "telegram"),
		tgbot.WithDefaultHandler(botHandler.DefaultHandler()),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create telegram bot: %w", err)
	}

	botHandler.SetBot(tgNotif.Bot())
	botHandler.RegisterHandlers()

	multi := notifier.NewMultiNotifier(store, logger.With("component", "notifier"))
	if err := multi.Register("telegram", tgNotif); err != nil {
		return nil, nil, nil, fmt.Errorf("register telegram notifier: %w", err)
	}

	// Register web push notifier when VAPID keys are configured and the storage
	// backend implements the PushSubscriptionStore interface (added in PR #898).
	if cfg.Push.VAPIDPublicKey != "" && cfg.Push.VAPIDPrivateKey != "" {
		if subStore, ok := store.(webpush.SubscriptionStore); ok {
			wpNotif := webpush.New(subStore, cfg.Push.VAPIDPublicKey, cfg.Push.VAPIDPrivateKey, cfg.Push.VAPIDSubject, logger.With("component", "webpush"))
			if err := multi.Register("webpush", wpNotif); err != nil {
				return nil, nil, nil, fmt.Errorf("register webpush notifier: %w", err)
			}
			logger.Info("webpush notifier registered")
		} else {
			logger.Warn("VAPID keys configured but storage does not support push subscriptions yet")
		}
	}

	return botHandler, tgNotif, multi, nil
}

func buildAPI(cfg *config.Config, store storage.Store, dynCatalog *catalog.DynamicCatalog, logHub *logstream.Hub, logLevel *slog.LevelVar, logger *slog.Logger, fetchers *fetcher.Factory, plSvc *pricelist.Service) (*api.Server, error) {
	var firebaseAuth api.TokenVerifier
	if cfg.Firebase.ProjectID != "" {
		v, err := api.NewFirebaseVerifier(cfg.Firebase.CredentialsFile, cfg.Firebase.CredentialsJSON, cfg.Firebase.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("init firebase: %w", err)
		}
		firebaseAuth = v
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
		Logger:       logger,
		API:          cfg.API,
		FirebaseAuth: firebaseAuth,
		BotUsername:  cfg.Telegram.BotUsername,
		LogHub:       logHub,
		LogLevel:     logLevel,
		Fetchers:     fetchers,
		PriceListSvc: plSvc,
		Bind:         cfg.HTTP.Bind,
	})

	return apiServer, nil
}

func buildHTTPServer(cfg *config.Config, h *health.Status, apiServer *api.Server, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.PublicHandler())
	mux.Handle("/api/v1/", apiServer.Routes())
	mux.Handle("/", spa.Handler(web.DistFS()))
	srv := &http.Server{
		Addr:              cfg.HTTP.Bind,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server failed", "error", err)
		}
	}()
	return srv
}

func newLogHandler(format string, level slog.Leveler) slog.Handler {
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

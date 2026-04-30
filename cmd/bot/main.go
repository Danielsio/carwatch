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
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/notifier/telegram"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/spa"
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
	logger = slog.New(newLogHandler(cfg.LogFormat, logLevel))
	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	store, err := sqlite.New(cfg.Storage.DBPath)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer func() { _ = store.Close() }()

	var proxyPool *fetcher.ProxyPool
	if len(cfg.HTTP.Proxies) > 0 {
		proxyPool = fetcher.NewProxyPool(cfg.HTTP.Proxies)
	}

	var yad2Fetcher *yad2.Yad2Fetcher
	if proxyPool != nil {
		yad2Fetcher, err = yad2.NewFetcherWithProxyPool(cfg.HTTP.UserAgents, proxyPool, logger)
	} else {
		yad2Fetcher, err = yad2.NewFetcher(cfg.HTTP.UserAgents, cfg.HTTP.Proxy, logger)
	}
	if err != nil {
		return fmt.Errorf("create fetcher: %w", err)
	}

	paginatingFetcher := fetcher.NewPaginatingFetcher(yad2Fetcher, cfg.HTTP.MaxPages)
	cachingFetcher := fetcher.NewCachingFetcher(paginatingFetcher, 5*time.Minute)
	yad2CB := fetcher.NewCircuitBreaker(cachingFetcher, 5, 30*time.Minute)

	dynCatalog := catalog.NewDynamic(store, logger)
	dynCatalog.Load(context.Background())

	var winwinFetcher *winwin.WinWinFetcher
	if proxyPool != nil {
		winwinFetcher, err = winwin.NewFetcherWithProxyPool(cfg.HTTP.UserAgents, proxyPool, logger)
	} else {
		winwinFetcher, err = winwin.NewFetcher(cfg.HTTP.UserAgents, cfg.HTTP.Proxy, logger)
	}
	if err != nil {
		return fmt.Errorf("create winwin fetcher: %w", err)
	}
	cachingWinwin := fetcher.NewCachingFetcher(winwinFetcher, 5*time.Minute)
	winwinCB := fetcher.NewCircuitBreaker(cachingWinwin, 5, 30*time.Minute)

	fetcherFactory := fetcher.NewFactory()
	fetcherFactory.Register("yad2", yad2CB)
	fetcherFactory.Register("winwin", winwinCB)

	h := health.New()
	h.SetUserCounter(store)
	h.SetSearchCounter(store)

	botHandler := cwbot.New(nil, store, store, cwbot.Config{
		AdminChatID:  cfg.Telegram.AdminChatID,
		MaxSearches:  cfg.Telegram.MaxSearches,
		BotUsername:   cfg.Telegram.BotUsername,
		PollInterval: cfg.Polling.Interval,
		Health:       h,
		Digests:      store,
		Listings:     store,
		Saved:        store,
		Hidden:       store,
		DailyDigests: store,
		Catalog:      dynCatalog,
	}, logger)

	tgNotif, err := telegram.New(cfg.Telegram.Token, logger,
		tgbot.WithDefaultHandler(botHandler.DefaultHandler()),
	)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	botHandler.SetBot(tgNotif.Bot())
	botHandler.RegisterHandlers()

	multi := notifier.NewMultiNotifier(store, logger)
	if err := multi.Register("telegram", tgNotif); err != nil {
		return fmt.Errorf("register telegram notifier: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := multi.Connect(ctx); err != nil {
		return fmt.Errorf("connect notifiers: %w", err)
	}
	defer func() { _ = multi.Disconnect() }()

	var firebaseAuth api.TokenVerifier
	if cfg.Firebase.ProjectID != "" {
		v, err := api.NewFirebaseVerifier(cfg.Firebase.CredentialsFile, cfg.Firebase.CredentialsJSON, cfg.Firebase.ProjectID)
		if err != nil {
			return fmt.Errorf("init firebase: %w", err)
		}
		firebaseAuth = v
	}

	apiServer := api.New(api.Config{
		Catalog:  dynCatalog,
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Admin:    store,
		Saved:    store,
		Hidden:   store,
		Notifs:   store,
		Logger:   logger,
		API:      cfg.API,
		FirebaseAuth: firebaseAuth,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Handler())
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
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("http server shutdown failed", "error", err)
		}
	}()

	sched, err := scheduler.NewWithOptions(cfg, cachingFetcher, store, multi, logger, scheduler.Options{
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
		MarketStore:      store,
		DailyDigestStore: store,
	})
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	botHandler.SetPollTrigger(sched)
	botHandler.StartCleanup(ctx)

	go tgNotif.Bot().Start(ctx)
	logger.Info("bot started",
		"health", "http://"+cfg.HTTP.Bind+"/healthz",
	)

	return sched.Run(ctx)
}

func newLogHandler(format string, level slog.Level) slog.Handler {
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


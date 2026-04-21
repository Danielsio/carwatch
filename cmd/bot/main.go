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

	cwbot "github.com/dsionov/carwatch/internal/bot"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/dashboard"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/fetcher/winwin"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/notifier/telegram"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
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

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

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

	logLevel, _ := config.ParseLogLevel(cfg.LogLevel)
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	logger.Info("config loaded", "log_level", cfg.LogLevel, "version", version)

	store, err := sqlite.New(cfg.Storage.DBPath)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer store.Close()

	var yad2Fetcher *yad2.Yad2Fetcher
	if len(cfg.HTTP.Proxies) > 0 {
		pool := fetcher.NewProxyPool(cfg.HTTP.Proxies)
		yad2Fetcher, err = yad2.NewFetcherWithProxyPool(cfg.HTTP.UserAgents, pool, logger)
	} else {
		yad2Fetcher, err = yad2.NewFetcher(cfg.HTTP.UserAgents, cfg.HTTP.Proxy, logger)
	}
	if err != nil {
		return fmt.Errorf("create fetcher: %w", err)
	}

	cachingFetcher := fetcher.NewCachingFetcher(yad2Fetcher, 5*time.Minute)

	dynCatalog := catalog.NewDynamic(store, logger)
	dynCatalog.Load(context.Background())

	var winwinFetcher *winwin.WinWinFetcher
	if len(cfg.HTTP.Proxies) > 0 {
		pool := fetcher.NewProxyPool(cfg.HTTP.Proxies)
		winwinFetcher, err = winwin.NewFetcherWithProxyPool(cfg.HTTP.UserAgents, pool, logger)
	} else {
		winwinFetcher, err = winwin.NewFetcher(cfg.HTTP.UserAgents, cfg.HTTP.Proxy, logger)
	}
	if err != nil {
		return fmt.Errorf("create winwin fetcher: %w", err)
	}
	cachingWinwin := fetcher.NewCachingFetcher(winwinFetcher, 5*time.Minute)

	fetcherFactory := fetcher.NewFactory()
	fetcherFactory.Register("yad2", cachingFetcher)
	fetcherFactory.Register("winwin", cachingWinwin)

	h := health.New()
	h.SetUserCounter(store)
	h.SetSearchCounter(store)

	botHandler := cwbot.New(nil, store, store, cwbot.Config{
		AdminChatID: cfg.Telegram.AdminChatID,
		MaxSearches: cfg.Telegram.MaxSearches,
		BotUsername:  cfg.Telegram.BotUsername,
		Health:      h,
		Catalog:     dynCatalog,
	}, logger)

	tgNotif, err := telegram.New(cfg.Telegram.Token, logger,
		tgbot.WithDefaultHandler(botHandler.DefaultHandler()),
	)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	botHandler.SetBot(tgNotif.Bot())
	botHandler.RegisterHandlers()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := tgNotif.Connect(ctx); err != nil {
		return fmt.Errorf("connect telegram: %w", err)
	}

	dash := dashboard.NewHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Handler())
	mux.Handle("/dashboard", dash)
	srv := &http.Server{
		Addr:              ":8080",
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

	sched, err := scheduler.NewWithOptions(cfg, cachingFetcher, store, tgNotif, logger, scheduler.Options{
		Health:          h,
		Queue:           store,
		Prices:          store,
		ConfigPath:      configPath,
		FetcherFactory:  fetcherFactory,
		ListingStore:    store,
		SearchStore:     store,
		CatalogIngester: dynCatalog,
	})
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	go tgNotif.Bot().Start(ctx)
	logger.Info("bot started",
		"health", ":8080/healthz",
		"dashboard", ":8080/dashboard",
	)

	return sched.Run(ctx)
}

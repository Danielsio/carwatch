// Command scraper runs the scheduler, global feed fetching, and Percolator
// without the REST API, SPA, or Telegram bot. It optionally publishes alerts
// to Redis for the notifier worker, or delivers them directly as a fallback.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dsionov/carwatch/internal/app"
	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/scheduler"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	healthBind := flag.String("health-bind", "0.0.0.0:8081", "health endpoint bind address")
	flag.Parse()

	if *showVersion {
		fmt.Printf("scraper %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
		return
	}

	logger := slog.New(app.NewLogHandler("auto", slog.LevelInfo))

	if err := run(*configPath, *healthBind, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath, healthBind string, logger *slog.Logger) error {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, _, err = app.SetupLogger(cfg)
	if err != nil {
		return err
	}
	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required for the scraper; alerts must go through the Redis-backed notifier worker for rate limiting")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	telShutdown, err := app.InitTelemetry(ctx, "carwatch-scraper", version, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = telShutdown(context.Background()) }()

	store, err := app.OpenStore(cfg)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer func() { _ = store.Close() }()

	fb, err := app.BuildFetchers(cfg, logger)
	if err != nil {
		return err
	}

	dynCatalog := app.BuildDynamicCatalog(ctx, fb.Yad2, logger)

	h := health.New()
	h.SetVersion(version)

	// Health endpoint on a separate port.
	healthSrv := app.BuildHealthServer(healthBind, h, logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("health server shutdown failed", "error", err)
		}
	}()

	pub, err := broker.NewPublisher(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return fmt.Errorf("create redis publisher: %w", err)
	}
	defer func() { _ = pub.Close() }()
	logger.Info("redis publisher enabled", "addr", cfg.Redis.Addr)

	// The scheduler needs a notifier for non-alert messages (e.g., digest
	// delivery). Alerts go through the Redis publisher to the notifier worker.
	multi, err := app.BuildMultiNotifier(cfg, store, store, logger)
	if err != nil {
		return err
	}
	if err := multi.Connect(ctx); err != nil {
		return fmt.Errorf("connect notifiers: %w", err)
	}
	defer func() { _ = multi.Disconnect() }()

	plSvc, plCleanup, err := app.BuildPriceListService(cfg, store, logger)
	if err != nil {
		return err
	}
	defer plCleanup()

	kmEnricher := yad2.NewEnricher(fb.Yad2, logger.With("component", "enricher"), yad2.EnricherConfig{})

	var n notifier.Notifier = multi
	sched, err := scheduler.NewWithOptions(cfg, fb.Caching, store, n, logger.With("component", "scheduler"), scheduler.Options{
		Observer:         h,
		Prices:           store,
		ConfigPath:       configPath,
		FetcherFactory:   fb.Factory,
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
		PriceListSvc:     plSvc,
		DailyDigestStore: store,
		CycleLogStore:    store,
		Publisher:        pub,
	})
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	logger.Info("scraper started",
		"health", "http://"+healthBind+"/healthz",
	)

	h.MarkSchedulerStarted()
	return sched.Run(ctx)
}

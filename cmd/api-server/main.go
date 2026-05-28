// Command api-server runs the REST API and SPA. It does not run the Telegram
// bot poller, scheduler, or Redis consumer — those are separate binaries.
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
	"github.com/dsionov/carwatch/internal/cwlog"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/logstream"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	migrateOnly := flag.Bool("migrate-only", false, "run database migrations and exit")
	skipMigrate := flag.Bool("skip-migrate", false, "skip auto-migration on startup")
	flag.Parse()

	if *showVersion {
		fmt.Printf("api-server %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
		return
	}

	if *migrateOnly {
		cfg, err := app.LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load config: %v\n", err)
			os.Exit(1)
		}
		store, err := app.OpenStore(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
			os.Exit(1)
		}
		_ = store.Close()
		fmt.Println("migrations complete")
		return
	}

	logger := slog.New(app.NewLogHandler("auto", slog.LevelInfo))

	if err := run(*configPath, *skipMigrate, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, skipMigrate bool, logger *slog.Logger) error {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, logLevelVar, err := app.SetupLogger(cfg)
	if err != nil {
		return err
	}

	logHub := logstream.NewHub(10000)
	baseHandler := logger.Handler()
	teeHandler := logstream.NewTeeHandler(baseHandler, logHub,
		"yad2", "scheduler", "enricher",
		"api-pricelist", "bot", "telegram", "notifier",
		"circuit_breaker",
	)
	logger = slog.New(cwlog.NewContextHandler(teeHandler))
	slog.SetDefault(logger)

	if cfg.Redis.Addr != "" {
		logSub, subErr := logstream.NewRedisSubscriber(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, logHub, logger)
		if subErr != nil {
			logger.Warn("redis log subscriber failed, only api-server logs will be visible", "error", subErr)
		} else {
			defer func() { _ = logSub.Close() }()
			logger.Info("subscribed to cross-service log stream", "redis", cfg.Redis.Addr)
		}
	}

	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	telShutdown, err := app.InitTelemetry(ctx, "carwatch-api", version, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = telShutdown(context.Background()) }()

	store, err := app.OpenStore(cfg, skipMigrate)
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
	h.SetUserCounter(store)
	h.SetSearchCounter(store)
	h.SetDBSizer(store)

	plSvc, plCleanup, err := app.BuildPriceListService(cfg, store, logger)
	if err != nil {
		return err
	}
	defer plCleanup()

	apiServer, err := app.BuildAPI(cfg, store, dynCatalog, logger, fb.Factory, plSvc, logHub, logLevelVar)
	if err != nil {
		return err
	}
	defer apiServer.Shutdown()

	srv := app.BuildHTTPServer(cfg, h, apiServer, logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("http server shutdown failed", "error", err)
		}
	}()

	logger.Info("api-server started",
		"health", "http://"+cfg.HTTP.Bind+"/healthz",
	)

	// Block until shutdown signal.
	<-ctx.Done()
	logger.Info("api-server shutting down")
	return nil
}

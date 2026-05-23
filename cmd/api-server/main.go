// Command api-server runs the REST API, SPA, and Telegram bot without the
// scheduler or Redis consumer. Use this when the scraper and notifier run
// as separate processes.
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
	"github.com/dsionov/carwatch/internal/health"
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
		fmt.Printf("api-server %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
		return
	}

	logger := slog.New(app.NewLogHandler("auto", slog.LevelInfo))

	if err := run(*configPath, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, logger *slog.Logger) error {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, _, err = app.SetupLogger(cfg)
	if err != nil {
		return err
	}
	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	telShutdown, err := app.InitTelemetry(ctx, "carwatch-api", version, cfg, logger)
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
	h.SetUserCounter(store)
	h.SetSearchCounter(store)
	h.SetDBSizer(store)

	bb, err := app.BuildBot(cfg, store, dynCatalog, h, logger)
	if err != nil {
		return err
	}

	if err := bb.Multi.Connect(ctx); err != nil {
		return fmt.Errorf("connect notifiers: %w", err)
	}
	defer func() { _ = bb.Multi.Disconnect() }()

	plSvc, plCleanup, err := app.BuildPriceListService(cfg, store, logger)
	if err != nil {
		return err
	}
	defer plCleanup()

	apiServer, err := app.BuildAPI(cfg, store, dynCatalog, logger, fb.Factory, plSvc)
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

	// TODO: migrate to webhook-based Telegram integration for multi-replica
	// deployments. Long-polling only works with a single api-server replica
	// because getUpdates is exclusive to one consumer per bot token.
	// Start Telegram bot polling in a goroutine with restart-on-failure.
	bb.Handler.StartCleanup(ctx)
	go func() {
		const maxBackoff = 30 * time.Second
		backoff := time.Second
		for {
			h.MarkBotPollingAlive()
			logger.Info("telegram bot polling loop starting")
			bb.TgNotifier.Bot().Start(ctx)
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

	logger.Info("api-server started",
		"health", "http://"+cfg.HTTP.Bind+"/healthz",
	)

	// Block until shutdown signal.
	<-ctx.Done()
	logger.Info("api-server shutting down")
	return nil
}

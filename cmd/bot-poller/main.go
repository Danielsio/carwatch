// Command bot-poller runs the Telegram bot long-polling loop without the
// REST API, scheduler, or Redis consumer. Use this alongside api-server,
// scraper, and notifier as separate processes.
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
	healthBind := flag.String("health-bind", "0.0.0.0:8082", "health endpoint bind address")
	flag.Parse()

	if *showVersion {
		fmt.Printf("bot-poller %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
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

	if cfg.Redis.Addr != "" {
		logPub, pubErr := logstream.NewRedisPublisher(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		if pubErr != nil {
			logger.Warn("redis log publisher failed", "error", pubErr)
		} else {
			defer func() { _ = logPub.Close() }()
			teeHandler := logstream.NewTeeHandler(logger.Handler(), logPub, "bot", "telegram")
			logger = slog.New(cwlog.NewContextHandler(teeHandler))
			slog.SetDefault(logger)
		}
	}

	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	telShutdown, err := app.InitTelemetry(ctx, "carwatch-bot-poller", version, cfg, logger)
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

	bb, err := app.BuildBot(cfg, store, dynCatalog, h, logger)
	if err != nil {
		return err
	}

	if err := bb.Multi.Connect(ctx); err != nil {
		return fmt.Errorf("connect notifiers: %w", err)
	}
	defer func() { _ = bb.Multi.Disconnect() }()

	healthSrv := app.BuildHealthServer(healthBind, h, logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("health server shutdown failed", "error", err)
		}
	}()

	bb.Handler.StartCleanup(ctx)

	logger.Info("bot-poller started",
		"health", "http://"+healthBind+"/healthz",
	)

	pollingLoop(ctx, h, func(c context.Context) { bb.TgNotifier.Bot().Start(c) }, logger)
	return nil
}

type backoffConfig struct {
	initial        time.Duration
	max            time.Duration
	resetThreshold time.Duration
}

var defaultBackoff = backoffConfig{
	initial:        time.Second,
	max:            30 * time.Second,
	resetThreshold: 30 * time.Second,
}

func pollingLoop(ctx context.Context, h *health.Status, poll func(context.Context), logger *slog.Logger) {
	pollingLoopWithConfig(ctx, h, poll, logger, defaultBackoff)
}

func pollingLoopWithConfig(ctx context.Context, h *health.Status, poll func(context.Context), logger *slog.Logger, bo backoffConfig) {
	backoff := bo.initial
	for {
		h.MarkBotPollingAlive()
		logger.Info("telegram bot polling loop starting")
		start := time.Now()
		poll(ctx)
		if ctx.Err() != nil {
			logger.Info("bot-poller shutting down")
			return
		}
		if time.Since(start) >= bo.resetThreshold {
			backoff = bo.initial
		}
		logger.Error("telegram bot polling loop exited unexpectedly, restarting", "backoff", backoff.String())
		h.MarkBotPollingDead()
		select {
		case <-ctx.Done():
			logger.Info("bot-poller shutting down")
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, bo.max)
	}
}

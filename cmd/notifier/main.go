// Command notifier runs the Redis consumer that reads alerts from the
// carwatch:alerts stream and delivers notifications via Telegram and WebPush.
// It does not need PostgreSQL for listing data, but it does need a user store
// to resolve notification channels and a push subscription store for WebPush.
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
	"github.com/dsionov/carwatch/internal/health"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

const defaultHealthBind = "0.0.0.0:8083"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	healthBind := flag.String("health-bind", defaultHealthBind, "health endpoint bind address")
	flag.Parse()

	if *showVersion {
		fmt.Printf("notifier %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	telShutdown, err := app.InitTelemetry(ctx, "carwatch-notifier", version, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = telShutdown(context.Background()) }()

	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required for the notifier worker")
	}

	// The notifier worker needs a user store for channel resolution and a
	// push subscription store for WebPush delivery. Both come from PostgreSQL.
	store, err := app.OpenStore(cfg)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer func() { _ = store.Close() }()

	multi, err := app.BuildMultiNotifier(cfg, store, store, logger)
	if err != nil {
		return err
	}
	if err := multi.Connect(ctx); err != nil {
		return fmt.Errorf("connect notifiers: %w", err)
	}
	defer func() { _ = multi.Disconnect() }()

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

	cons, err := broker.NewConsumer(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB,
		multi.NotifyRaw, logger.With("component", "broker-consumer"))
	if err != nil {
		return fmt.Errorf("create redis consumer: %w", err)
	}
	defer func() { _ = cons.Close() }()

	logger.Info("notifier worker started",
		"redis", cfg.Redis.Addr,
		"health", "http://"+healthBind+"/healthz",
	)

	// Run consumer with restart-on-failure.
	// On SIGTERM: stop reading new messages, let in-flight deliveries finish
	// within a 10-second grace period.
	const maxBackoff = 30 * time.Second
	backoff := time.Second
	for {
		if err := cons.Run(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Info("notifier worker draining in-flight deliveries")
				drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
				cons.Drain(drainCtx)
				drainCancel()
				logger.Info("notifier worker shut down")
				return nil
			}
			logger.Error("redis consumer exited, restarting", "backoff", backoff.String(), "error", err)
			select {
			case <-ctx.Done():
				logger.Info("notifier worker shut down")
				return nil
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
		}
	}
}

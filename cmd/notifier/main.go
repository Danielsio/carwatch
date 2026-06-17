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
	"github.com/dsionov/carwatch/internal/cwlog"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/logstream"
	"github.com/dsionov/carwatch/internal/storage"
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
	skipMigrate := flag.Bool("skip-migrate", false, "skip auto-migration on startup")
	flag.Parse()

	if *showVersion {
		fmt.Printf("notifier %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
		return
	}

	logger := slog.New(app.NewLogHandler("auto", slog.LevelInfo))

	if err := run(*configPath, *healthBind, *skipMigrate, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath, healthBind string, skipMigrate bool, logger *slog.Logger) error {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, _, err = app.SetupLogger(cfg)
	if err != nil {
		return err
	}

	baseHandler := logger.Handler()
	logPub, pubErr := logstream.NewRedisPublisher(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if pubErr != nil {
		logger.Warn("redis log publisher failed", "error", pubErr)
	} else {
		defer func() { _ = logPub.Close() }()
		baseHandler = logstream.NewTeeHandler(baseHandler, logPub,
			"notifier", "telegram", "webpush", "broker-consumer",
		)
	}
	logger = slog.New(cwlog.NewContextHandler(baseHandler))
	slog.SetDefault(logger)

	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required for the notifier worker")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	telShutdown, err := app.InitTelemetry(ctx, "carwatch-notifier", version, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = telShutdown(context.Background()) }()

	// The notifier worker needs a user store for channel resolution and a
	// push subscription store for WebPush delivery. Both come from PostgreSQL.
	store, err := app.OpenStore(cfg, skipMigrate)
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
		multi.NotifyRaw,
		logger.With("component", "broker-consumer"),
		broker.WithDeadLetterHook(releaseDedupClaimsOnDeadLetter(store, logger)),
	)
	if err != nil {
		return fmt.Errorf("create redis consumer: %w", err)
	}
	defer func() { _ = cons.Close() }()

	logger.Info("notifier worker started",
		"redis", cfg.Redis.Addr,
		"health", "http://"+healthBind+"/healthz",
	)

	consumerLoop(ctx, cons, logger)
	return nil
}

type runner interface {
	Run(ctx context.Context) error
	Drain(ctx context.Context)
}

type consumerBackoff struct {
	initial        time.Duration
	max            time.Duration
	resetThreshold time.Duration
}

var defaultConsumerBackoff = consumerBackoff{
	initial:        time.Second,
	max:            30 * time.Second,
	resetThreshold: 30 * time.Second,
}

func consumerLoop(ctx context.Context, cons runner, logger *slog.Logger) {
	consumerLoopWithConfig(ctx, cons, logger, defaultConsumerBackoff)
}

func consumerLoopWithConfig(ctx context.Context, cons runner, logger *slog.Logger, bo consumerBackoff) {
	backoff := bo.initial
	for {
		start := time.Now()
		if err := cons.Run(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Info("notifier worker draining in-flight deliveries")
				drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
				cons.Drain(drainCtx)
				drainCancel()
				logger.Info("notifier worker shut down")
				return
			}
			if time.Since(start) >= bo.resetThreshold {
				backoff = bo.initial
			}
			logger.Error("redis consumer exited, restarting", "backoff", backoff.String(), "error", err)
			select {
			case <-ctx.Done():
				logger.Info("notifier worker shut down")
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, bo.max)
		}
	}
}

func releaseDedupClaimsOnDeadLetter(dedup storage.DedupStore, logger *slog.Logger) broker.DeadLetterHook {
	return func(ctx context.Context, alert broker.Alert) error {
		if dedup == nil || len(alert.Tokens) == 0 {
			return nil
		}
		for _, token := range alert.Tokens {
			cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
			err := dedup.ReleaseClaim(cleanupCtx, token, alert.ChatID, alert.SearchID)
			cleanupCancel()
			if err != nil {
				return err
			}
		}
		logger.Warn("released dedup claims for dead-lettered alert",
			"chat_id", alert.ChatID,
			"tokens", len(alert.Tokens),
			"search_id", alert.SearchID)
		return nil
	}
}

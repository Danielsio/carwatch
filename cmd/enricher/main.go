// Command enricher runs a standalone worker that consumes enrichment
// requests from the carwatch:enrich Redis stream and fetches individual
// listing pages from Yad2 to fill in missing km/city/image data.
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
	"github.com/dsionov/carwatch/internal/enricher"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/logstream"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

const defaultHealthBind = "0.0.0.0:8084"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	healthBind := flag.String("health-bind", defaultHealthBind, "health endpoint bind address")
	flag.Parse()

	if *showVersion {
		fmt.Printf("enricher %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
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

	baseHandler := logger.Handler()
	if cfg.Redis.Addr != "" {
		logPub, pubErr := logstream.NewRedisPublisher(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		if pubErr != nil {
			logger.Warn("redis log publisher failed", "error", pubErr)
		} else {
			defer func() { _ = logPub.Close() }()
			baseHandler = logstream.NewTeeHandler(baseHandler, logPub, "enricher")
		}
	}
	logger = slog.New(cwlog.NewContextHandler(baseHandler))
	slog.SetDefault(logger)

	logger.Info("config loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat, "version", version)

	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required for the enricher worker")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	telShutdown, err := app.InitTelemetry(ctx, "carwatch-enricher", version, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = telShutdown(context.Background()) }()

	store, err := app.OpenStore(cfg)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Build fetcher for item detail pages.
	fb, err := app.BuildFetchers(cfg, logger)
	if err != nil {
		return err
	}

	// Wrap Yad2Fetcher to satisfy enricher.ItemFetcher interface.
	itemFetcher := &yad2ItemFetcherAdapter{fetcher: fb.Yad2}

	// Create adaptive rate limiter.
	rl := enricher.NewAdaptiveRateLimiter(
		cfg.Enricher.BaseDelay,
		cfg.Enricher.MaxDelay,
		cfg.Enricher.CooldownDuration,
	)

	// Create the enrichment worker.
	worker := enricher.NewWorker(itemFetcher, store, rl, logger.With("component", "enricher"))

	h := health.New()
	h.SetVersion(version)

	healthSrv := app.BuildHealthServer(healthBind, h, logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("health server shutdown failed", "error", err)
		}
	}()

	cons, err := broker.NewEnrichConsumer(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB,
		worker.HandleRequest, logger.With("component", "enrich-consumer"))
	if err != nil {
		return fmt.Errorf("create enrich consumer: %w", err)
	}
	defer func() { _ = cons.Close() }()

	logger.Info("enricher worker started",
		"redis", cfg.Redis.Addr,
		"health", "http://"+healthBind+"/healthz",
		"base_delay", cfg.Enricher.BaseDelay,
		"max_delay", cfg.Enricher.MaxDelay,
		"cooldown", cfg.Enricher.CooldownDuration,
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
	backoff := defaultConsumerBackoff.initial
	for {
		start := time.Now()
		if err := cons.Run(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Info("enricher worker draining in-flight requests")
				drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
				cons.Drain(drainCtx)
				drainCancel()
				logger.Info("enricher worker shut down")
				return
			}
			if time.Since(start) >= defaultConsumerBackoff.resetThreshold {
				backoff = defaultConsumerBackoff.initial
			}
			logger.Error("enrich consumer exited, restarting", "backoff", backoff.String(), "error", err)
			select {
			case <-ctx.Done():
				logger.Info("enricher worker shut down")
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, defaultConsumerBackoff.max)
		}
	}
}

// yad2ItemFetcherAdapter adapts yad2.Yad2Fetcher to the enricher.ItemFetcher interface.
type yad2ItemFetcherAdapter struct {
	fetcher *yad2.Yad2Fetcher
}

func (a *yad2ItemFetcherAdapter) FetchItem(ctx context.Context, token string) (enricher.ItemDetails, error) {
	details, err := a.fetcher.FetchItem(ctx, token)
	if err != nil {
		return enricher.ItemDetails{}, err
	}
	return enricher.ItemDetails{
		Km:       details.Km,
		ImageURL: details.ImageURL,
		City:     details.City,
		Area:     details.Area,
	}, nil
}

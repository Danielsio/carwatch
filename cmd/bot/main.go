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

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/notifier/whatsapp"
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

	bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(*configPath, bootstrapLogger); err != nil {
		bootstrapLogger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, bootstrapLogger *slog.Logger) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logLevel, _ := config.ParseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	logger.Info("config loaded", "searches", len(cfg.Searches), "log_level", cfg.LogLevel)

	fetcher, err := yad2.NewFetcher(cfg.HTTP.UserAgents, cfg.HTTP.Proxy, logger)
	if err != nil {
		return fmt.Errorf("create fetcher: %w", err)
	}

	store, err := sqlite.New(cfg.Storage.DBPath)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer store.Close()

	notif := whatsapp.New(cfg.WhatsApp.DBPath, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := notif.Connect(ctx); err != nil {
		return fmt.Errorf("connect notifier: %w", err)
	}
	defer func() { _ = notif.Disconnect() }()

	h := health.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Handler())
	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server failed", "error", err)
		}
	}()
	defer srv.Close()

	sched, err := scheduler.New(cfg, fetcher, store, notif, logger, h)
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	logger.Info("bot starting", "health_endpoint", ":8080/healthz")
	return sched.Run(ctx)
}

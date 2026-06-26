package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Polling   PollingConfig   `yaml:"polling"`
	Telegram  TelegramConfig  `yaml:"telegram"`
	Storage   StorageConfig   `yaml:"storage"`
	HTTP      HTTPConfig      `yaml:"http"`
	API       APIConfig       `yaml:"api"`
	Firebase  FirebaseConfig  `yaml:"firebase"`
	Push      PushConfig      `yaml:"push"`
	Redis     RedisConfig     `yaml:"redis"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
	Enricher  EnricherConfig  `yaml:"enricher"`
	LogLevel  string          `yaml:"log_level"`
	LogFormat string          `yaml:"log_format"`
}

type EnricherConfig struct {
	BaseDelay           time.Duration `yaml:"base_delay"`
	MaxDelay            time.Duration `yaml:"max_delay"`
	CooldownDuration    time.Duration `yaml:"cooldown_duration"`
	MaxPerMinute        int           `yaml:"max_per_minute"`
	MaxAttemptsPerToken int           `yaml:"max_attempts_per_token"`
	BackfillInterval    time.Duration `yaml:"backfill_interval"`
	InlineMaxPerCycle   int           `yaml:"inline_max_per_cycle"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`     // default "localhost:6379"
	Password string `yaml:"password"` // default ""
	DB       int    `yaml:"db"`       // default 0
}

type TelemetryConfig struct {
	TracesExporter string `yaml:"traces_exporter"` // "none" (default), "stdout", "otlp"
	OTLPEndpoint   string `yaml:"otlp_endpoint"`   // e.g. "localhost:4317"
	MetricsPath    string `yaml:"metrics_path"`    // default "/metrics"
	AuthToken      string `yaml:"auth_token"`      // required for non-local metrics exposure
}

type PushConfig struct {
	VAPIDPublicKey  string `yaml:"vapid_public_key"`
	VAPIDPrivateKey string `yaml:"vapid_private_key"`
	VAPIDSubject    string `yaml:"vapid_subject"`
}

type FirebaseConfig struct {
	CredentialsFile string `yaml:"credentials_file"`
	CredentialsJSON string `yaml:"credentials_json"`
	ProjectID       string `yaml:"project_id"`
}

type APIConfig struct {
	CORSOrigins []string `yaml:"cors_origins"`
	// TrustForwardedFor, when true, uses X-Forwarded-For (leftmost hop) for IP rate limiting.
	// Enable only behind a trusted reverse proxy that overwrites client-controlled forwarded headers.
	TrustForwardedFor    bool   `yaml:"trust_forwarded_for"`
	DevChatID            int64  `yaml:"dev_chat_id"`
	AuthToken            string `yaml:"auth_token"`
	AdminChatID          int64  `yaml:"-"` // derived from telegram.admin_chat_id at startup
	AdminEmail           string `yaml:"admin_email"`
	MaxSearches          int    `yaml:"-"` // derived from telegram.max_searches at startup
	MaxConcurrentFetches int    `yaml:"max_concurrent_fetches"`
	// AllowInsecureDevAuth permits the API to start with the unauthenticated
	// dev-auth fallback on a non-localhost bind address (no Firebase verifier).
	// This is an explicit, dangerous opt-in for local container development;
	// production must never set it. Without it, a non-local bind without a
	// configured verifier is a hard startup error.
	AllowInsecureDevAuth bool `yaml:"allow_insecure_dev_auth"`
}

type PollingConfig struct {
	Interval             time.Duration `yaml:"interval"`
	Jitter               time.Duration `yaml:"jitter"`
	ActiveHours          *ActiveHours  `yaml:"active_hours"`
	Timezone             string        `yaml:"timezone"`
	MaxConcurrentFetches int           `yaml:"max_concurrent_fetches"`
	EnrichGraceSeconds   int           `yaml:"enrich_grace_seconds"`
}

type ActiveHours struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

type TelegramConfig struct {
	Token                  string `yaml:"token"`
	AdminChatID            int64  `yaml:"admin_chat_id"`
	MaxSearches            int    `yaml:"max_searches"`
	BotUsername            string `yaml:"bot_username"`
	QuickStartManufacturer int    `yaml:"quick_start_manufacturer"`
	QuickStartModel        int    `yaml:"quick_start_model"`
}

type StorageConfig struct {
	Driver         string        `yaml:"driver"`
	DSN            string        `yaml:"dsn"`
	MigrationsPath string        `yaml:"migrations_path"`
	PruneAfter     time.Duration `yaml:"prune_after"`
}

type HTTPConfig struct {
	Bind       string   `yaml:"bind"`
	UserAgents []string `yaml:"user_agents"`
	Proxy      string   `yaml:"proxy"`
	Proxies    []string `yaml:"proxies"`
	MaxPages   int      `yaml:"max_pages"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	raw := string(data)
	data = []byte(os.ExpandEnv(raw))

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	warnHardcodedSecrets(raw)

	return cfg, nil
}

func warnHardcodedSecrets(raw string) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return
	}
	checkSecret := func(path, envHint string, section map[string]any, key string) {
		val, _ := section[key].(string)
		val = strings.TrimSpace(val)
		if val != "" && !strings.Contains(val, "${") {
			slog.Warn(path + " appears hardcoded in config; use ${" + envHint + "} for production")
		}
	}
	if tg, ok := doc["telegram"].(map[string]any); ok {
		checkSecret("telegram.token", "TELEGRAM_BOT_TOKEN", tg, "token")
	}
	if redis, ok := doc["redis"].(map[string]any); ok {
		checkSecret("redis.password", "REDIS_PASSWORD", redis, "password")
	}
	if push, ok := doc["push"].(map[string]any); ok {
		checkSecret("push.vapid_private_key", "VAPID_PRIVATE", push, "vapid_private_key")
	}
	if storage, ok := doc["storage"].(map[string]any); ok {
		checkSecret("storage.dsn", "DATABASE_URL", storage, "dsn")
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Polling.Interval == 0 {
		cfg.Polling.Interval = 15 * time.Minute
	}
	if cfg.Polling.Jitter == 0 {
		cfg.Polling.Jitter = 5 * time.Minute
	}
	if cfg.Polling.Timezone == "" {
		cfg.Polling.Timezone = "Asia/Jerusalem"
	}
	if cfg.Storage.Driver == "" {
		cfg.Storage.Driver = "postgres"
	}
	if cfg.Storage.MigrationsPath == "" {
		cfg.Storage.MigrationsPath = "./migrations"
	}
	if cfg.Storage.PruneAfter == 0 {
		cfg.Storage.PruneAfter = 30 * 24 * time.Hour
	}
	if cfg.HTTP.Bind == "" {
		cfg.HTTP.Bind = "127.0.0.1:8080"
	}
	if len(cfg.HTTP.UserAgents) == 0 {
		cfg.HTTP.UserAgents = defaultUserAgents()
	}
	if cfg.Telegram.MaxSearches == 0 {
		cfg.Telegram.MaxSearches = 10
	}
	if cfg.Polling.MaxConcurrentFetches <= 0 {
		cfg.Polling.MaxConcurrentFetches = 4
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "auto"
	}
	if cfg.Telemetry.TracesExporter == "" {
		cfg.Telemetry.TracesExporter = "none"
	}
	if cfg.Telemetry.MetricsPath == "" {
		cfg.Telemetry.MetricsPath = "/metrics"
	}
	if cfg.Enricher.BaseDelay == 0 {
		cfg.Enricher.BaseDelay = 500 * time.Millisecond
	}
	if cfg.Enricher.MaxDelay == 0 {
		cfg.Enricher.MaxDelay = 10 * time.Minute
	}
	if cfg.Enricher.CooldownDuration == 0 {
		cfg.Enricher.CooldownDuration = 15 * time.Minute
	}
	if cfg.Enricher.MaxPerMinute == 0 {
		cfg.Enricher.MaxPerMinute = 60
	}
	if cfg.Enricher.MaxAttemptsPerToken == 0 {
		cfg.Enricher.MaxAttemptsPerToken = 10
	}
	if cfg.Enricher.BackfillInterval == 0 {
		cfg.Enricher.BackfillInterval = 5 * time.Minute
	}
	if cfg.Enricher.InlineMaxPerCycle == 0 {
		cfg.Enricher.InlineMaxPerCycle = 15
	}
	if cfg.API.MaxConcurrentFetches <= 0 {
		cfg.API.MaxConcurrentFetches = 10
	}
	var filtered []string
	for _, o := range cfg.API.CORSOrigins {
		o = strings.TrimSpace(o)
		if o != "" && o != "https://" && o != "http://" {
			filtered = append(filtered, o)
		}
	}
	cfg.API.CORSOrigins = filtered
	if len(cfg.API.CORSOrigins) == 0 {
		cfg.API.CORSOrigins = []string{"http://localhost:5173"}
	}
}

func validate(cfg *Config) error {
	fb := cfg.Firebase
	hasCreds := fb.CredentialsFile != "" || fb.CredentialsJSON != ""
	if fb.ProjectID == "" && hasCreds {
		return fmt.Errorf("firebase.project_id is required when credentials are set")
	}
	if fb.ProjectID != "" && !hasCreds {
		return fmt.Errorf("firebase.credentials_file or firebase.credentials_json is required when project_id is set")
	}

	if cfg.Polling.Interval <= 0 {
		return fmt.Errorf("polling.interval must be positive, got %s", cfg.Polling.Interval)
	}
	if cfg.Polling.Jitter < 0 {
		return fmt.Errorf("polling.jitter must be non-negative, got %s", cfg.Polling.Jitter)
	}

	if ah := cfg.Polling.ActiveHours; ah != nil {
		if _, err := parseTimeOfDay(ah.Start); err != nil {
			return fmt.Errorf("active_hours.start %q: must be HH:MM format", ah.Start)
		}
		if _, err := parseTimeOfDay(ah.End); err != nil {
			return fmt.Errorf("active_hours.end %q: must be HH:MM format", ah.End)
		}
	}
	switch cfg.Storage.Driver {
	case "postgres":
	default:
		return fmt.Errorf("storage.driver %q: must be postgres", cfg.Storage.Driver)
	}
	if strings.TrimSpace(cfg.Storage.DSN) == "" {
		return fmt.Errorf("storage.dsn is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", cfg.HTTP.Bind); err != nil {
		return fmt.Errorf("http.bind %q: must be a valid host:port", cfg.HTTP.Bind)
	}
	if _, err := ParseLogLevel(cfg.LogLevel); err != nil {
		return fmt.Errorf("log_level %q: must be debug, info, warn, or error", cfg.LogLevel)
	}
	switch cfg.LogFormat {
	case "auto", "json", "pretty":
	default:
		return fmt.Errorf("log_format %q: must be auto, json, or pretty", cfg.LogFormat)
	}
	if cfg.Telegram.MaxSearches < 0 {
		return fmt.Errorf("telegram.max_searches must be >= 0")
	}
	if cfg.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	switch cfg.Telemetry.TracesExporter {
	case "none", "stdout", "otlp":
	default:
		return fmt.Errorf("telemetry.traces_exporter %q: must be none, stdout, or otlp", cfg.Telemetry.TracesExporter)
	}
	if cfg.Redis.Addr != "" {
		if _, err := net.ResolveTCPAddr("tcp", cfg.Redis.Addr); err != nil {
			return fmt.Errorf("redis.addr %q: must be a valid host:port", cfg.Redis.Addr)
		}
		if cfg.Redis.DB < 0 {
			return fmt.Errorf("redis.db must be >= 0, got %d", cfg.Redis.DB)
		}
	}

	if cfg.Enricher.MaxDelay < cfg.Enricher.BaseDelay {
		return fmt.Errorf("enricher.max_delay (%s) must be >= enricher.base_delay (%s)", cfg.Enricher.MaxDelay, cfg.Enricher.BaseDelay)
	}

	for _, origin := range cfg.API.CORSOrigins {
		u, err := url.Parse(origin)
		if err != nil {
			return fmt.Errorf("api.cors_origins: invalid URL %q", origin)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("api.cors_origins: %q must have a scheme and host (e.g. https://example.com)", origin)
		}
		if u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("api.cors_origins: %q must be a bare origin (scheme://host[:port]), no path or query", origin)
		}
	}
	return nil
}

func ParseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level: %s", level)
	}
}

func parseTimeOfDay(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

func defaultUserAgents() []string {
	return []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36",
	}
}

package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Polling  PollingConfig  `yaml:"polling"`
	Telegram TelegramConfig `yaml:"telegram"`
	Storage  StorageConfig  `yaml:"storage"`
	HTTP     HTTPConfig     `yaml:"http"`
	LogLevel string         `yaml:"log_level"`
}

type PollingConfig struct {
	Interval             time.Duration `yaml:"interval"`
	Jitter               time.Duration `yaml:"jitter"`
	ActiveHours          *ActiveHours  `yaml:"active_hours"`
	Timezone             string        `yaml:"timezone"`
	MaxConcurrentFetches int           `yaml:"max_concurrent_fetches"`
}

type ActiveHours struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

type SourceParams struct {
	Manufacturer int `yaml:"manufacturer"`
	Model        int `yaml:"model"`
	YearMin      int `yaml:"year_min"`
	YearMax      int `yaml:"year_max"`
	PriceMin     int `yaml:"price_min"`
	PriceMax     int `yaml:"price_max"`
	Page         int `yaml:"-"`
}

type FilterCriteria struct {
	YearMin     int      `yaml:"year_min"`
	YearMax     int      `yaml:"year_max"`
	PriceMax    int      `yaml:"price_max"`
	EngineMinCC float64  `yaml:"engine_min_cc"`
	EngineMaxCC float64  `yaml:"engine_max_cc"`
	MaxKm       int      `yaml:"max_km"`
	MaxHand     int      `yaml:"max_hand"`
	Keywords    []string `yaml:"keywords"`
	ExcludeKeys []string `yaml:"exclude_keys"`
}

type TelegramConfig struct {
	Token       string `yaml:"token"`
	AdminChatID int64  `yaml:"admin_chat_id"`
	MaxSearches int    `yaml:"max_searches"`
	BotUsername string `yaml:"bot_username"`
}

type StorageConfig struct {
	DBPath     string        `yaml:"db_path"`
	PruneAfter time.Duration `yaml:"prune_after"`
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

	data = []byte(os.ExpandEnv(string(data)))

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
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
	if cfg.Storage.DBPath == "" {
		cfg.Storage.DBPath = "./data/dedup.db"
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
		cfg.Telegram.MaxSearches = 3
	}
	if cfg.Polling.MaxConcurrentFetches <= 0 {
		cfg.Polling.MaxConcurrentFetches = 4
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
}

func validate(cfg *Config) error {
	if ah := cfg.Polling.ActiveHours; ah != nil {
		if _, err := parseTimeOfDay(ah.Start); err != nil {
			return fmt.Errorf("active_hours.start %q: must be HH:MM format", ah.Start)
		}
		if _, err := parseTimeOfDay(ah.End); err != nil {
			return fmt.Errorf("active_hours.end %q: must be HH:MM format", ah.End)
		}
	}
	if _, err := net.ResolveTCPAddr("tcp", cfg.HTTP.Bind); err != nil {
		return fmt.Errorf("http.bind %q: must be a valid host:port", cfg.HTTP.Bind)
	}
	if _, err := ParseLogLevel(cfg.LogLevel); err != nil {
		return fmt.Errorf("log_level %q: must be debug, info, warn, or error", cfg.LogLevel)
	}
	if cfg.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
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
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	}
}

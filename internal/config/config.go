package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Polling  PollingConfig  `yaml:"polling"`
	Searches []SearchConfig `yaml:"searches"`
	WhatsApp WhatsAppConfig `yaml:"whatsapp"`
	Telegram TelegramConfig `yaml:"telegram"`
	Storage  StorageConfig  `yaml:"storage"`
	HTTP     HTTPConfig     `yaml:"http"`
	LogLevel string         `yaml:"log_level"`
	Notifier string         `yaml:"notifier"`
}

type PollingConfig struct {
	Interval    time.Duration `yaml:"interval"`
	Jitter      time.Duration `yaml:"jitter"`
	ActiveHours *ActiveHours  `yaml:"active_hours"`
	Timezone    string        `yaml:"timezone"`
}

type ActiveHours struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

type SearchConfig struct {
	Name       string         `yaml:"name"`
	Source     string         `yaml:"source"`
	Params     SourceParams   `yaml:"params"`
	Filters    FilterCriteria `yaml:"filters"`
	Recipients []string       `yaml:"recipients"`
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
	EngineMinCC float64  `yaml:"engine_min_cc"`
	EngineMaxCC float64  `yaml:"engine_max_cc"`
	MaxKm       int      `yaml:"max_km"`
	MaxHand     int      `yaml:"max_hand"`
	Keywords    []string `yaml:"keywords"`
	ExcludeKeys []string `yaml:"exclude_keys"`
}

type WhatsAppConfig struct {
	DBPath string `yaml:"db_path"`
}

type TelegramConfig struct {
	Token string `yaml:"token"`
}

type StorageConfig struct {
	DBPath     string        `yaml:"db_path"`
	PruneAfter time.Duration `yaml:"prune_after"`
}

type HTTPConfig struct {
	UserAgents []string `yaml:"user_agents"`
	Proxy      string   `yaml:"proxy"`
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
	if cfg.WhatsApp.DBPath == "" {
		cfg.WhatsApp.DBPath = "./data/whatsapp.db"
	}
	if cfg.Storage.DBPath == "" {
		cfg.Storage.DBPath = "./data/dedup.db"
	}
	if cfg.Storage.PruneAfter == 0 {
		cfg.Storage.PruneAfter = 30 * 24 * time.Hour
	}
	if len(cfg.HTTP.UserAgents) == 0 {
		cfg.HTTP.UserAgents = defaultUserAgents()
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.Notifier == "" {
		cfg.Notifier = "whatsapp"
	}
}

func validate(cfg *Config) error {
	if len(cfg.Searches) == 0 {
		return fmt.Errorf("at least one search must be configured")
	}
	for i, s := range cfg.Searches {
		if s.Name == "" {
			return fmt.Errorf("search[%d]: name is required", i)
		}
		if s.Source == "" {
			return fmt.Errorf("search[%d] %q: source is required", i, s.Name)
		}
		if len(s.Recipients) == 0 {
			return fmt.Errorf("search[%d] %q: at least one recipient is required", i, s.Name)
		}
		if s.Filters.EngineMinCC > 0 && s.Filters.EngineMinCC < 100 {
			return fmt.Errorf("search[%d] %q: engine_min_cc=%.0f looks like liters, expected cc (e.g. 1800)", i, s.Name, s.Filters.EngineMinCC)
		}
		if s.Filters.EngineMaxCC > 0 && s.Filters.EngineMaxCC < 100 {
			return fmt.Errorf("search[%d] %q: engine_max_cc=%.0f looks like liters, expected cc (e.g. 2100)", i, s.Name, s.Filters.EngineMaxCC)
		}
	}
	if ah := cfg.Polling.ActiveHours; ah != nil {
		if _, err := parseTimeOfDay(ah.Start); err != nil {
			return fmt.Errorf("active_hours.start %q: must be HH:MM format", ah.Start)
		}
		if _, err := parseTimeOfDay(ah.End); err != nil {
			return fmt.Errorf("active_hours.end %q: must be HH:MM format", ah.End)
		}
	}
	if _, err := ParseLogLevel(cfg.LogLevel); err != nil {
		return fmt.Errorf("log_level %q: must be debug, info, warn, or error", cfg.LogLevel)
	}
	switch cfg.Notifier {
	case "whatsapp", "telegram":
	default:
		return fmt.Errorf("notifier %q: must be whatsapp or telegram", cfg.Notifier)
	}
	if cfg.Notifier == "telegram" && cfg.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required when notifier is telegram")
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

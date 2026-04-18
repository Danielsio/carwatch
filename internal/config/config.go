package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Polling  PollingConfig  `yaml:"polling"`
	Searches []SearchConfig `yaml:"searches"`
	WhatsApp WhatsAppConfig `yaml:"whatsapp"`
	Storage  StorageConfig  `yaml:"storage"`
	HTTP     HTTPConfig     `yaml:"http"`
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
}

type FilterCriteria struct {
	EngineMin   float64  `yaml:"engine_min"`
	EngineMax   float64  `yaml:"engine_max"`
	MaxKm       int      `yaml:"max_km"`
	MaxHand     int      `yaml:"max_hand"`
	Keywords    []string `yaml:"keywords"`
	ExcludeKeys []string `yaml:"exclude_keys"`
}

type WhatsAppConfig struct {
	DBPath string `yaml:"db_path"`
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

	cfg := &Config{
		Polling: PollingConfig{
			Interval: 15 * time.Minute,
			Jitter:   5 * time.Minute,
			Timezone: "Asia/Jerusalem",
		},
		WhatsApp: WhatsAppConfig{
			DBPath: "./data/whatsapp.db",
		},
		Storage: StorageConfig{
			DBPath:     "./data/dedup.db",
			PruneAfter: 30 * 24 * time.Hour,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
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
	}
	if len(cfg.HTTP.UserAgents) == 0 {
		cfg.HTTP.UserAgents = defaultUserAgents()
	}
	return nil
}

func defaultUserAgents() []string {
	return []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	}
}

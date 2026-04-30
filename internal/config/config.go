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
	LogLevel  string          `yaml:"log_level"`
	LogFormat string          `yaml:"log_format"`
}

type FirebaseConfig struct {
	CredentialsFile string `yaml:"credentials_file"`
	CredentialsJSON string `yaml:"credentials_json"`
	ProjectID       string `yaml:"project_id"`
}

type APIConfig struct {
	CORSOrigins []string `yaml:"cors_origins"`
	DevChatID   int64    `yaml:"dev_chat_id"`
	AuthToken   string   `yaml:"auth_token"`
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
	MaxPages int `yaml:"max_pages"`
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
	tg, ok := doc["telegram"].(map[string]any)
	if !ok {
		return
	}
	token, _ := tg["token"].(string)
	token = strings.TrimSpace(token)
	if token != "" && !strings.Contains(token, "${") {
		slog.Warn("telegram.token appears hardcoded in config; use ${TELEGRAM_BOT_TOKEN} for production")
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
	if cfg.LogFormat == "" {
		cfg.LogFormat = "auto"
	}
	filtered := cfg.API.CORSOrigins[:0]
	for _, o := range cfg.API.CORSOrigins {
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
	switch cfg.LogFormat {
	case "auto", "json", "pretty":
	default:
		return fmt.Errorf("log_format %q: must be auto, json, or pretty", cfg.LogFormat)
	}
	if cfg.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	for _, origin := range cfg.API.CORSOrigins {
		if _, err := url.Parse(origin); err != nil {
			return fmt.Errorf("api.cors_origins: invalid URL %q", origin)
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
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	}
}

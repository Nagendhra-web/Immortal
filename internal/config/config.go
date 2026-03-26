package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Name       string          `json:"name"`
	Mode       string          `json:"mode"` // ghost, reactive, predictive, autonomous
	DataDir    string          `json:"data_dir"`
	LogLevel   string          `json:"log_level"`
	Collectors CollectorConfig `json:"collectors"`
	Healing    HealingConfig   `json:"healing"`
	Security   SecurityConfig  `json:"security"`
	API        APIConfig       `json:"api"`
}

type CollectorConfig struct {
	LogFiles       []string      `json:"log_files"`
	MetricInterval time.Duration `json:"metric_interval"`
	Processes      []string      `json:"processes"`
	HTTPEndpoints  []string      `json:"http_endpoints"`
}

type HealingConfig struct {
	Enabled    bool          `json:"enabled"`
	MaxRetries int           `json:"max_retries"`
	Cooldown   time.Duration `json:"cooldown"`
	GhostMode  bool          `json:"ghost_mode"`
}

type SecurityConfig struct {
	Firewall   bool `json:"firewall"`
	RateLimit  int  `json:"rate_limit"`
	AntiScrape bool `json:"anti_scrape"`
	SecretScan bool `json:"secret_scan"`
	RASP       bool `json:"rasp"`
	ZeroTrust  bool `json:"zero_trust"`
}

type APIConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Host    string `json:"host"`
}

func Default() *Config {
	return &Config{
		Name:     "immortal",
		Mode:     "reactive",
		DataDir:  defaultDataDir(),
		LogLevel: "info",
		Collectors: CollectorConfig{
			MetricInterval: 10 * time.Second,
		},
		Healing: HealingConfig{
			Enabled:    true,
			MaxRetries: 3,
			Cooldown:   30 * time.Second,
		},
		Security: SecurityConfig{
			Firewall:   true,
			RateLimit:  100,
			AntiScrape: true,
			SecretScan: true,
			RASP:       true,
			ZeroTrust:  true,
		},
		API: APIConfig{
			Enabled: true,
			Port:    7777,
			Host:    "127.0.0.1",
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file: %w", err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse: %w", err)
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("config: name is required")
	}
	switch c.Mode {
	case "ghost", "reactive", "predictive", "autonomous":
	default:
		return fmt.Errorf("config: invalid mode '%s'", c.Mode)
	}
	if c.API.Port < 1 || c.API.Port > 65535 {
		return fmt.Errorf("config: invalid port %d", c.API.Port)
	}
	return nil
}

func defaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".immortal")
}

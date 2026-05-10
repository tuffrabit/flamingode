package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the top-level application configuration.
type Config struct {
	Providers    map[string]Provider `json:"providers"`
	DefaultModel string              `json:"defaultModel,omitempty"`
	Tools        ToolsConfig         `json:"tools,omitempty"`
	Debug        bool                `json:"-"`
	DebugLogPath string              `json:"-"`
}

// ToolsConfig holds configuration for individual tools.
type ToolsConfig struct {
	ReadFile ReadFileToolConfig `json:"read_file,omitempty"`
}

// ReadFileToolConfig holds configuration for the read_file tool.
type ReadFileToolConfig struct {
	MaxSize int64 `json:"max_size,omitempty"`
}

// Provider describes a custom inference endpoint.
type Provider struct {
	BaseURL string  `json:"baseUrl"`
	API     string  `json:"api"`
	APIKey  string  `json:"apiKey"`
	Models  []Model `json:"models"`
}

// Model describes a model available through a provider.
type Model struct {
	ID            string   `json:"id"`
	Name          string   `json:"name,omitempty"`
	Reasoning     bool     `json:"reasoning,omitempty"`
	Input         []string `json:"input,omitempty"`
	ContextWindow int      `json:"contextWindow,omitempty"`
	MaxTokens     int      `json:"maxTokens,omitempty"`
	Cost          *Cost    `json:"cost,omitempty"`
}

// Cost describes the per-token pricing for a model.
type Cost struct {
	Input     float64 `json:"input,omitempty"`
	Output    float64 `json:"output,omitempty"`
	CacheRead float64 `json:"cacheRead,omitempty"`
	CacheWrite float64 `json:"cacheWrite,omitempty"`
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	return filepath.Join(home, ".flamingode", "config.json"), nil
}

// defaultConfig returns a Config populated with sensible defaults.
func defaultConfig() Config {
	return Config{
		Providers: map[string]Provider{},
		Tools: ToolsConfig{
			ReadFile: ReadFileToolConfig{
				MaxSize: 100000,
			},
		},
	}
}

// ensureDir creates the config directory if it does not exist.
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("unable to create config directory: %w", err)
	}
	return nil
}

// writeDefault writes a default config to the given path.
func writeDefault(path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	data, err := json.MarshalIndent(defaultConfig(), "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal default config: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("unable to write default config: %w", err)
	}
	return nil
}

// Load reads the config from disk. If the config file does not exist, it
// creates the directory and writes a default config before returning it.
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := writeDefault(path); err != nil {
			return Config{}, err
		}
		return defaultConfig(), nil
	} else if err != nil {
		return Config{}, fmt.Errorf("unable to stat config file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("unable to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("unable to parse config file: %w", err)
	}

	return cfg, nil
}

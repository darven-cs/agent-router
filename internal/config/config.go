package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServiceConfig holds service-level configuration
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `yaml:"port"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"` // default model for all requests
}

// UpstreamConfig holds upstream provider configuration
type UpstreamConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	APIKey   string `yaml:"api_key"`
	AuthType string `yaml:"auth_type"` // "bearer" or "x-api-key"
	Enabled  bool   `yaml:"enabled"`
	Timeout  int    `yaml:"timeout"` // seconds
	Model    string `yaml:"model"`   // model name to use for this upstream (if empty, uses request model)
}

// Config holds the complete service configuration
type Config struct {
	Service   ServiceConfig    `yaml:"service"`
	Upstreams []UpstreamConfig `yaml:"upstreams"`
}

// LoadConfig reads and parses the YAML configuration file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Apply environment variable expansion
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveConfig marshals the Config struct to YAML and writes to the specified path.
// This enables persisting runtime TUI changes to config.yaml so they survive SIGHUP reload.
func SaveConfig(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

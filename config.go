package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ServiceConfig holds service-level configuration
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `yaml:"port"`
	APIKey  string `yaml:"api_key"`
}

// UpstreamConfig holds upstream provider configuration
type UpstreamConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	APIKey   string `yaml:"api_key"`
	AuthType string `yaml:"auth_type"` // "bearer" or "x-api-key"
	Enabled  bool   `yaml:"enabled"`
	Timeout  int    `yaml:"timeout"` // seconds
}

// Config holds the complete service configuration
type Config struct {
	Service   ServiceConfig   `yaml:"service"`
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

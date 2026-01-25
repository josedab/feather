// Package config provides configuration management for feather-cli.
package config

import (
	"os"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds the CLI configuration.
type Config struct {
	ServerURL    string        `yaml:"server_url" mapstructure:"server_url"`
	APIKey       string        `yaml:"api_key" mapstructure:"api_key"`
	OutputFormat string        `yaml:"output_format" mapstructure:"output_format"`
	Timeout      time.Duration `yaml:"timeout" mapstructure:"timeout"`
	Verbose      bool          `yaml:"verbose" mapstructure:"verbose"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		ServerURL:    "http://localhost:8080",
		OutputFormat: "table",
		Timeout:      30 * time.Second,
		Verbose:      false,
	}
}

// Load loads configuration from file and environment variables.
func Load(cfgFile string) (*Config, error) {
	cfg := DefaultConfig()

	// If config file specified, try to load it
	if cfgFile != "" {
		data, err := os.ReadFile(cfgFile)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Override with viper values (from env vars and flags)
	if v := viper.GetString("server_url"); v != "" {
		cfg.ServerURL = v
	}
	if v := viper.GetString("api_key"); v != "" {
		cfg.APIKey = v
	}
	if v := viper.GetString("output_format"); v != "" {
		cfg.OutputFormat = v
	}
	if viper.IsSet("verbose") {
		cfg.Verbose = viper.GetBool("verbose")
	}
	if v := viper.GetDuration("timeout"); v != 0 {
		cfg.Timeout = v
	}

	return cfg, nil
}

// Package config provides configuration loading and validation for the feature store.
//
// It supports loading configuration from YAML files and environment variables,
// with automatic type conversion and validation. The package defines the complete
// configuration schema including server settings, storage options, TLS configuration,
// ingestion sources, and observability settings.
//
// Configuration can be loaded via:
//
//	cfg, err := config.LoadFromFile("feather.yaml")
//	// or from environment variables:
//	cfg := config.LoadFromEnv()
package config

// Package logging provides structured logging for the feature store.
//
// It wraps the standard library slog package with feature store-specific
// configuration including log levels, output formats (JSON/text), and
// contextual attributes. The package enables consistent logging across
// all components.
//
// Usage:
//
//	logger := logging.New(logging.Config{Level: "info", Format: "json"})
//	slog.SetDefault(logger)
package logging

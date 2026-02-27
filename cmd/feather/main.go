// feather starts the Feather feature store server.
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/feather-store/feather/internal/core/config"
	"github.com/feather-store/feather/internal/core/logging"
)

// version is set at build time via -ldflags "-X main.version=<value>".
var version = "dev"

// configSource records where configuration was loaded from (for the startup banner).
var configSource string

//go:embed default_config.yaml
var defaultConfigData []byte

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Print version and exit")
	validateOnly := flag.Bool("validate", false, "Validate config and exit")
	verbose := flag.Bool("verbose", false, "Enable debug logging (overrides config log level)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("feather %s\n", version)
		return
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadFromFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Available config files:")
				fmt.Fprintln(os.Stderr, "  configs/feather-dev.yaml   — local development (start here)")
				fmt.Fprintln(os.Stderr, "  configs/feather-local.yaml — local with disk persistence")
				fmt.Fprintln(os.Stderr, "  configs/feather.yaml       — production reference")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Or run without -config to use environment variable defaults.")
			}
			os.Exit(1)
		}
		configSource = *configPath
	} else {
		// Auto-detect dev config when no -config flag is given
		devConfig := "configs/feather-dev.yaml"
		if _, statErr := os.Stat(devConfig); statErr == nil {
			fmt.Fprintf(os.Stderr, "No config specified, using %s (dev defaults)\n", devConfig)
			cfg, err = config.LoadFromFile(devConfig)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
				os.Exit(1)
			}
			configSource = devConfig
		} else {
			// Fall back to embedded dev config (works after go install)
			fmt.Fprintln(os.Stderr, "No config specified, using built-in development defaults")
			fmt.Fprintln(os.Stderr, "  Tip: pass -config <file> to use a custom configuration")
			cfg, err = config.LoadFromBytes(defaultConfigData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load embedded config: %v\n", err)
				os.Exit(1)
			}
			configSource = "built-in defaults"
		}
	}

	// Validate-only mode: check config and exit
	if *validateOnly {
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Config validation failed:\n%v\n", err)
			os.Exit(1)
		}
		featureCount := 0
		for _, g := range cfg.Schema.Groups {
			featureCount += len(g.Features)
		}
		fmt.Printf("✅ Config is valid (%s)\n", configSource)
		fmt.Printf("   HTTP: :%d  gRPC: :%d\n", cfg.Serving.HTTP.Port, cfg.Serving.GRPC.Port)
		fmt.Printf("   Schema: %d groups, %d features\n", len(cfg.Schema.Groups), featureCount)
		return
	}

	// Override log level if -verbose flag is set
	if *verbose {
		cfg.Logging.Level = "debug"
	}

	// Initialize logger first
	logger := logging.New(logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})
	slog.SetDefault(logger)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run the server
	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

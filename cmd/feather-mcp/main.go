// feather-mcp is an MCP (Model Context Protocol) server for Feather Feature Store.
// It allows AI assistants to interact with the feature store through a standardized interface.
//
// Usage:
//
//	feather-mcp [options]
//
// This server reads JSON-RPC requests from stdin and writes responses to stdout,
// following the MCP specification.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/config"
	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/core/vector"
	"github.com/feather-store/feather/internal/tools/mcp"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	logLevel := flag.String("log-level", "error", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Initialize logger (to stderr so it doesn't interfere with MCP protocol)
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	default:
		level = slog.LevelError
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadFromFile(*configPath)
		if err != nil {
			logger.Error("failed to load config", "error", err)
			os.Exit(1)
		}
	} else {
		cfg = config.LoadFromEnv()
	}

	// Create context with cancellation
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize components
	var store *storage.Store
	var schema *storage.Registry
	var agg *aggregation.Engine
	var vectorStore *vector.Store

	// Initialize schema registry
	schema = storage.NewRegistry()

	// Parse memory size
	maxMemory, err := config.ParseMemorySize(cfg.Storage.Hot.MaxMemory)
	if err != nil {
		logger.Warn("using default memory size", "error", err)
		maxMemory = 256 * 1024 * 1024 // 256MB default
	}

	// Initialize storage
	store, err = storage.NewStore(ctx, storage.StoreOptions{
		HotMaxSize:       maxMemory,
		WarmPath:         cfg.Storage.Warm.Path,
		WarmSyncInterval: cfg.Storage.Warm.SyncInterval,
		WarmInMemory:     cfg.Storage.Warm.Path == "" || cfg.Storage.Warm.Path == ":memory:",
		TTLCheckInterval: time.Minute,
	}, schema)
	if err != nil {
		logger.Error("failed to create store", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("store close error", "error", err)
		}
	}()

	// Initialize aggregation engine
	agg = aggregation.NewEngine()

	// Initialize vector store
	vectorStore = vector.NewStore(vector.StoreConfig{
		DataDir: cfg.Storage.Warm.Path,
	})

	logger.Info("starting MCP server")

	// Create and run MCP server
	server := mcp.NewServer(mcp.ServerConfig{
		Store:       store,
		Schema:      schema,
		Aggregation: agg,
		Vectors:     vectorStore,
		Logger:      logger,
	})

	if err := server.Run(ctx); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "MCP server stopped")
}

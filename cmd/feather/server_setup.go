package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/feather-store/feather/internal/app"
	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/config"
	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/ingestion"
	"github.com/feather-store/feather/internal/core/metrics"
	"github.com/feather-store/feather/internal/core/server"
	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/core/tracing"
	"github.com/feather-store/feather/internal/core/vector"
	"github.com/feather-store/feather/internal/integrations/dbt"
)

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("starting Feather Feature Store",
		"version", version,
		"http_port", cfg.Serving.HTTP.Port,
		"grpc_port", cfg.Serving.GRPC.Port,
	)

	// Initialize server manager for graceful shutdown
	serverMgr := app.NewServerManager(logger)

	// Initialize tracing
	var tracer *tracing.Tracer
	if cfg.Tracing.Enabled {
		var err error
		tracer, err = tracing.New(ctx, tracing.Config{
			Enabled:     cfg.Tracing.Enabled,
			Endpoint:    cfg.Tracing.Endpoint,
			ServiceName: cfg.Tracing.ServiceName,
			SampleRate:  cfg.Tracing.SampleRate,
			Insecure:    cfg.Tracing.Insecure,
		})
		if err != nil {
			logger.Warn("failed to initialize tracing", "error", err)
		} else {
			logger.Info("tracing enabled",
				"endpoint", cfg.Tracing.Endpoint,
				"sample_rate", cfg.Tracing.SampleRate,
			)
			defer func() {
				if err := tracer.Shutdown(ctx); err != nil {
					logger.Error("tracing shutdown error", "error", err)
				}
			}()
		}
	}

	// Initialize metrics
	var m *metrics.Metrics
	if cfg.Metrics.Prometheus.Enabled {
		m = metrics.NewMetrics("feather")
		stopRuntimeMetrics := m.StartRuntimeMetricsCollector(15 * time.Second)
		defer stopRuntimeMetrics()
	}

	// Initialize schema registry
	schema := storage.NewRegistry()

	// Register feature groups from config
	for _, groupCfg := range cfg.Schema.Groups {
		group := &domain.FeatureGroup{
			Name:        groupCfg.Name,
			EntityType:  groupCfg.EntityType,
			TTL:         groupCfg.TTL,
			Description: groupCfg.Description,
			Features:    make([]domain.FeatureSpec, 0, len(groupCfg.Features)),
		}

		for _, featureCfg := range groupCfg.Features {
			dataType, err := domain.ParseDataType(featureCfg.DataType)
			if err != nil {
				return fmt.Errorf("parsing data type for feature %s: %w", featureCfg.Name, err)
			}

			spec := domain.FeatureSpec{
				Name:       featureCfg.Name,
				DataType:   dataType,
				Dimensions: featureCfg.Dimensions,
				Default:    featureCfg.Default,
			}

			if featureCfg.Aggregation != nil {
				spec.Aggregation = &domain.AggregationSpec{
					Function: domain.AggFunction(featureCfg.Aggregation.Function),
					Window:   featureCfg.Aggregation.Window,
					SlideBy:  featureCfg.Aggregation.SlideBy,
				}
			}

			group.Features = append(group.Features, spec)
		}

		if err := schema.RegisterGroup(group); err != nil {
			logger.Warn("failed to register feature group",
				"group", groupCfg.Name,
				"error", err,
			)
		}
	}

	// Parse hot tier max memory
	maxMemory, err := config.ParseMemorySize(cfg.Storage.Hot.MaxMemory)
	if err != nil {
		return fmt.Errorf("parsing max memory: %w", err)
	}

	// Initialize storage
	store, err := storage.NewStore(ctx, storage.StoreOptions{
		HotMaxSize:       maxMemory,
		WarmPath:         cfg.Storage.Warm.Path,
		WarmSyncInterval: cfg.Storage.Warm.SyncInterval,
		WarmInMemory:     cfg.Storage.Warm.Path == "" || cfg.Storage.Warm.Path == ":memory:",
		TTLCheckInterval: time.Minute,
	}, schema)
	if err != nil {
		return fmt.Errorf("creating store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("store close error", "error", err)
		}
	}()

	// Initialize aggregation engine
	agg := aggregation.NewEngine()

	// Initialize vector store
	vectorStore := vector.NewStore(vector.StoreConfig{
		DataDir: cfg.Storage.Warm.Path,
	})
	logger.Info("vector store initialized")

	// Register aggregations from schema
	for _, group := range schema.ListGroups() {
		for _, feature := range group.Features {
			if feature.Aggregation != nil {
				agg.RegisterAggregation(feature.Name, feature.Aggregation)
			}
		}
	}

	// Start Prometheus metrics server
	if cfg.Metrics.Prometheus.Enabled {
		metricsServer := &http.Server{
			Addr:           fmt.Sprintf(":%d", cfg.Metrics.Prometheus.Port),
			Handler:        m.Handler(),
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		}

		// Configure TLS if enabled (fail-closed: TLS errors are fatal when TLS is enabled)
		if cfg.TLS.Enabled {
			tlsConfig, err := cfg.TLS.BuildTLSConfig()
			if err != nil {
				return fmt.Errorf("failed to build TLS config for metrics server: %w", err)
			}
			if tlsConfig != nil {
				metricsServer.TLSConfig = tlsConfig
			}
		}

		serverMgr.Register("metrics", &app.HTTPServerWrapper{Server: metricsServer})

		go func() {
			defer app.RecoverPanic(logger, "metrics-server")
			logger.Info("starting metrics server",
				"port", cfg.Metrics.Prometheus.Port,
				"tls", cfg.TLS.Enabled,
			)
			var err error
			if cfg.TLS.Enabled {
				err = metricsServer.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			} else {
				err = metricsServer.ListenAndServe()
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				app.LogListenError(logger, "metrics server", cfg.Metrics.Prometheus.Port, err)
			}
		}()
	}

	// Start HTTP server with health checker
	healthChecker := server.NewHealthChecker(store, agg, schema)
	httpServer, err := server.NewHTTPServer(
		ctx, store, agg, schema, m,
		server.HTTPServerConfig{
			Core: server.HTTPServerCoreConfig{
				Port:          cfg.Serving.HTTP.Port,
				ReadTimeout:   cfg.Serving.HTTP.ReadTimeout,
				WriteTimeout:  cfg.Serving.HTTP.WriteTimeout,
				HealthChecker: healthChecker,
				VectorStore:   vectorStore,
				TLS:           &cfg.TLS,
				Tracer:        tracer,
			},
			Features: server.HTTPServerFeatureConfig{
				EnabledFeatures: buildEnabledFeatures(cfg),
			},
			Dependencies: server.HTTPServerDependencies{
				DBTOptions: &dbt.SyncOptions{
					DefaultEntityType: cfg.DBT.DefaultEntityType,
					Owner:             cfg.DBT.Owner,
					Team:              cfg.DBT.Team,
					IncludeSources:    cfg.DBT.IncludeSources,
					IncludeMetrics:    cfg.DBT.IncludeMetrics,
					EntityTypeMapping: cfg.DBT.EntityTypeMapping,
				},
				// Handlers below require external dependencies not yet initialized
				// EnableDrift, EnableLineage, EnableSemantic, EnableWASM,
				// EnableFederation, EnableQuality, EnableGraphQL, EnableAutogen,
				// EnableExperiment can be enabled when their dependencies are provided
			},
		},
	)
	if err != nil {
		return fmt.Errorf("creating HTTP server: %w", err)
	}
	serverMgr.Register("http", httpServer)

	go func() {
		defer app.RecoverPanic(logger, "http-server")
		logger.Info("starting HTTP server", "port", cfg.Serving.HTTP.Port)
		if err := httpServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.LogListenError(logger, "HTTP server", cfg.Serving.HTTP.Port, err)
		}
	}()

	// Start gRPC server
	grpcServer := server.NewGRPCServer(
		store, agg, schema, m,
		server.GRPCServerConfig{
			Port:          cfg.Serving.GRPC.Port,
			MaxConcurrent: cfg.Serving.GRPC.MaxConcurrent,
			HealthChecker: healthChecker,
			TLS:           &cfg.TLS,
			Tracer:        tracer,
		},
	)

	go func() {
		defer app.RecoverPanic(logger, "grpc-server")
		logger.Info("starting gRPC server", "port", cfg.Serving.GRPC.Port)
		if err := grpcServer.Start(); err != nil {
			app.LogListenError(logger, "gRPC server", cfg.Serving.GRPC.Port, err)
		}
	}()

	// Start HTTP ingestion if enabled
	var ingestionServer *http.Server
	if cfg.Ingestion.HTTP.Enabled {
		httpIngestion := ingestion.NewHTTPIngestion(store, agg, schema)
		mux := http.NewServeMux()
		httpIngestion.RegisterRoutes(mux)

		ingestionServer = &http.Server{
			Addr:           fmt.Sprintf(":%d", cfg.Ingestion.HTTP.Port),
			Handler:        mux,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		}

		// Configure TLS if enabled (fail-closed: TLS errors are fatal when TLS is enabled)
		if cfg.TLS.Enabled {
			tlsConfig, err := cfg.TLS.BuildTLSConfig()
			if err != nil {
				return fmt.Errorf("failed to build TLS config for ingestion server: %w", err)
			}
			if tlsConfig != nil {
				ingestionServer.TLSConfig = tlsConfig
			}
		}

		serverMgr.Register("ingestion", &app.HTTPServerWrapper{Server: ingestionServer})

		go func() {
			defer app.RecoverPanic(logger, "ingestion-server")
			logger.Info("starting HTTP ingestion server",
				"port", cfg.Ingestion.HTTP.Port,
				"tls", cfg.TLS.Enabled,
			)
			var err error
			if cfg.TLS.Enabled {
				err = ingestionServer.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			} else {
				err = ingestionServer.ListenAndServe()
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				app.LogListenError(logger, "HTTP ingestion server", cfg.Ingestion.HTTP.Port, err)
			}
		}()
	}

	// Start Kafka consumer if enabled
	var kafkaConsumer *ingestion.KafkaConsumer
	if cfg.Ingestion.Kafka.Enabled {
		var err error
		kafkaConsumer, err = ingestion.NewKafkaConsumer(
			ingestion.KafkaConfig{
				Brokers:          cfg.Ingestion.Kafka.Brokers,
				Topic:            cfg.Ingestion.Kafka.Topic,
				ConsumerGroup:    cfg.Ingestion.Kafka.ConsumerGroup,
				SecurityProtocol: cfg.Ingestion.Kafka.Security.Protocol,
				SASLMechanism:    cfg.Ingestion.Kafka.Security.SASLMechanism,
				SASLUsername:     cfg.Ingestion.Kafka.Security.SASLUsername,
				SASLPassword:     cfg.Ingestion.Kafka.Security.SASLPassword,
				SSLCAFile:        cfg.Ingestion.Kafka.Security.SSLCAFile,
				SSLCertFile:      cfg.Ingestion.Kafka.Security.SSLCertFile,
				SSLKeyFile:       cfg.Ingestion.Kafka.Security.SSLKeyFile,
			},
			store, agg, logger,
		)
		if err != nil {
			logger.Warn("failed to create Kafka consumer", "error", err)
		} else {
			go func() {
				defer app.RecoverPanic(logger, "kafka-consumer")
				logger.Info("starting Kafka consumer",
					"brokers", cfg.Ingestion.Kafka.Brokers,
					"topic", cfg.Ingestion.Kafka.Topic,
				)
				if err := kafkaConsumer.Start(ctx); err != nil {
					logger.Error("Kafka consumer error", "error", err)
				}
			}()
		}
	}

	logger.Info("Feather Feature Store started successfully")
	logger.Info("endpoints",
		"http", fmt.Sprintf("http://localhost:%d", cfg.Serving.HTTP.Port),
		"grpc", fmt.Sprintf("localhost:%d", cfg.Serving.GRPC.Port),
		"health", fmt.Sprintf("http://localhost:%d/health", cfg.Serving.HTTP.Port),
		"metrics", fmt.Sprintf("http://localhost:%d/metrics", cfg.Metrics.Prometheus.Port),
	)

	if cfg.Logging.Format == "text" {
		printStartupBanner(cfg, configSource)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("received shutdown signal, initiating graceful shutdown")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown all managed servers in parallel
	serverMgr.ShutdownAll(shutdownCtx)

	// Stop gRPC server with timeout
	grpcServer.Stop(shutdownCtx)

	// Close Kafka consumer
	if kafkaConsumer != nil {
		if err := kafkaConsumer.Close(); err != nil {
			logger.Error("Kafka consumer close error", "error", err)
		}
	}

	// Store will be closed by defer

	logger.Info("Feather Feature Store stopped gracefully")
	return nil
}

// printStartupBanner prints a human-readable banner with service URLs.
// Only called when logging format is "text" (dev mode).
func printStartupBanner(cfg *config.Config, configSource string) {
	httpURL := fmt.Sprintf("http://localhost:%d", cfg.Serving.HTTP.Port)
	grpcAddr := fmt.Sprintf("localhost:%d", cfg.Serving.GRPC.Port)
	healthURL := fmt.Sprintf("%s/health", httpURL)
	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", cfg.Metrics.Prometheus.Port)

	// Count schema groups and features
	groups := len(cfg.Schema.Groups)
	features := 0
	for _, g := range cfg.Schema.Groups {
		features += len(g.Features)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  ╭─────────────────────────────────────────────────╮")
	fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("🪶 Feather Feature Store %s", version))
	fmt.Fprintln(os.Stderr, "  │                                                 │")
	fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("HTTP API:  %s", httpURL))
	fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("gRPC API:  %s", grpcAddr))
	if cfg.Metrics.Prometheus.Enabled {
		fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("Metrics:   %s", metricsURL))
	}
	fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("Health:    %s", healthURL))
	fmt.Fprintln(os.Stderr, "  │                                                 │")
	if configSource != "" {
		fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("Config:    %s", configSource))
	}
	fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("Schema:    %d group(s), %d feature(s)", groups, features))
	fmt.Fprintln(os.Stderr, "  │                                                 │")
	fmt.Fprintf(os.Stderr, "  │  %-48s│\n", fmt.Sprintf("Try: curl %s", healthURL))
	fmt.Fprintln(os.Stderr, "  │  Stop: Ctrl+C                                   │")
	fmt.Fprintln(os.Stderr, "  ╰─────────────────────────────────────────────────╯")
	fmt.Fprintln(os.Stderr)
}

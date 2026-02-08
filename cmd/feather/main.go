// feather starts the Feather feature store server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/config"
	"github.com/feather-store/feather/internal/dbt"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/ingestion"
	"github.com/feather-store/feather/internal/logging"
	"github.com/feather-store/feather/internal/metrics"
	"github.com/feather-store/feather/internal/server"
	"github.com/feather-store/feather/internal/storage"
	"github.com/feather-store/feather/internal/tracing"
	"github.com/feather-store/feather/internal/vector"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadFromFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	} else {
		cfg = config.LoadFromEnv()
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

// serverManager tracks all running servers for graceful shutdown.
type serverManager struct {
	mu      sync.Mutex
	servers map[string]shutdownable
	logger  *slog.Logger
}

type shutdownable interface {
	Shutdown(ctx context.Context) error
}

type httpServerWrapper struct {
	server *http.Server
}

func (h *httpServerWrapper) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

func newServerManager(logger *slog.Logger) *serverManager {
	return &serverManager{
		servers: make(map[string]shutdownable),
		logger:  logger,
	}
}

func (m *serverManager) register(name string, s shutdownable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[name] = s
}

func (m *serverManager) shutdownAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var wg sync.WaitGroup
	for name, s := range m.servers {
		wg.Add(1)
		go func(name string, s shutdownable) {
			defer wg.Done()
			m.logger.Info("shutting down server", "name", name)
			if err := s.Shutdown(ctx); err != nil {
				m.logger.Error("shutdown error", "name", name, "error", err)
			}
		}(name, s)
	}
	wg.Wait()
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("starting Feather Feature Store",
		"version", "1.0.0",
		"http_port", cfg.Serving.HTTP.Port,
		"grpc_port", cfg.Serving.GRPC.Port,
	)

	// Initialize server manager for graceful shutdown
	serverMgr := newServerManager(logger)

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
			spec := domain.FeatureSpec{
				Name:       featureCfg.Name,
				DataType:   domain.ParseDataType(featureCfg.DataType),
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
			Addr:         fmt.Sprintf(":%d", cfg.Metrics.Prometheus.Port),
			Handler:      m.Handler(),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
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

		serverMgr.register("metrics", &httpServerWrapper{metricsServer})

		go func() {
			defer recoverPanic(logger, "metrics-server")
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
				logger.Error("metrics server error", "error", err)
			}
		}()
	}

	// Start HTTP server with health checker
	healthChecker := server.NewHealthChecker(store, agg, schema)
	httpServer := server.NewHTTPServer(
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
				EnableGroups:        true,
				EnableBackfill:      true,
				EnableStreaming:     true,
				EnableCatalog:       true,
				EnableAuth:          true,
				EnableML:            true,
				EnableTransform:     true,
				EnableCache:         true,
				EnableConsistency:   true,
				EnableImpact:        true,
				EnableObservability: true,
				EnableBenchmark:     true,
				EnableUI:            cfg.UI.Enabled,
				EnableDBT:           cfg.DBT.Enabled,
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
	serverMgr.register("http", httpServer)

	go func() {
		defer recoverPanic(logger, "http-server")
		logger.Info("starting HTTP server", "port", cfg.Serving.HTTP.Port)
		if err := httpServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server error", "error", err)
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
		defer recoverPanic(logger, "grpc-server")
		logger.Info("starting gRPC server", "port", cfg.Serving.GRPC.Port)
		if err := grpcServer.Start(); err != nil {
			logger.Error("gRPC server error", "error", err)
		}
	}()

	// Start HTTP ingestion if enabled
	var ingestionServer *http.Server
	if cfg.Ingestion.HTTP.Enabled {
		httpIngestion := ingestion.NewHTTPIngestion(store, agg, schema)
		mux := http.NewServeMux()
		httpIngestion.RegisterRoutes(mux)

		ingestionServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Ingestion.HTTP.Port),
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
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

		serverMgr.register("ingestion", &httpServerWrapper{ingestionServer})

		go func() {
			defer recoverPanic(logger, "ingestion-server")
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
				logger.Error("HTTP ingestion server error", "error", err)
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
				defer recoverPanic(logger, "kafka-consumer")
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

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("received shutdown signal, initiating graceful shutdown")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown all managed servers in parallel
	serverMgr.shutdownAll(shutdownCtx)

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

// recoverPanic recovers from panics in goroutines and logs them.
func recoverPanic(logger *slog.Logger, component string) {
	if r := recover(); r != nil {
		stack := debug.Stack()
		logger.Error("panic recovered",
			"component", component,
			"panic", r,
			"stack", string(stack),
		)
	}
}

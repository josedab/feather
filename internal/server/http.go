package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/autogen"
	"github.com/feather-store/feather/internal/cluster"
	"github.com/feather-store/feather/internal/composition"
	"github.com/feather-store/feather/internal/config"
	"github.com/feather-store/feather/internal/cost"
	"github.com/feather-store/feather/internal/dbt"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/drift"
	"github.com/feather-store/feather/internal/experiment"
	"github.com/feather-store/feather/internal/federation"
	"github.com/feather-store/feather/internal/freshness"
	"github.com/feather-store/feather/internal/gitops"
	"github.com/feather-store/feather/internal/graphql"
	"github.com/feather-store/feather/internal/lineage"
	"github.com/feather-store/feather/internal/logging"
	"github.com/feather-store/feather/internal/metrics"
	"github.com/feather-store/feather/internal/migration"
	"github.com/feather-store/feather/internal/quality"
	"github.com/feather-store/feather/internal/saas"
	"github.com/feather-store/feather/internal/semantic"
	"github.com/feather-store/feather/internal/sla"
	"github.com/feather-store/feather/internal/storage"
	"github.com/feather-store/feather/internal/ui"
	"github.com/feather-store/feather/internal/vector"
	"github.com/feather-store/feather/internal/warehouse"
	"github.com/feather-store/feather/internal/wasm"
)

// HTTPServer provides HTTP REST API for feature serving.
type HTTPServer struct {
	store         *storage.Store
	aggregation   *aggregation.Engine
	schema        *storage.Registry
	metrics       *metrics.Metrics
	healthChecker *HealthChecker
	server        *http.Server
	mux           *http.ServeMux
	tlsConfig     *config.TLSConfig
}

// HTTPServerConfig configures the HTTP server.
type HTTPServerConfig struct {
	Port          int
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	HealthChecker *HealthChecker
	VectorStore   *vector.Store
	TLS           *config.TLSConfig
	CORS          *CORSConfig
	// MaxRequestSize is the maximum allowed request body size in bytes.
	// If 0, defaults to DefaultMaxRequestSize (1MB).
	MaxRequestSize int64
	// Optional handlers for extended functionality
	EnableGroups        bool
	EnableBackfill      bool
	EnableStreaming     bool
	EnableCatalog       bool
	EnableAuth          bool
	EnableCORS          bool
	EnableDrift         bool
	EnableLineage       bool
	EnableSemantic      bool
	EnableWASM          bool
	EnableFederation    bool
	EnableML            bool
	EnableTransform     bool
	EnableQuality       bool
	EnableCache         bool
	EnableConsistency   bool
	EnableImpact        bool
	EnableObservability bool
	EnableGraphQL       bool
	EnableAutogen       bool
	EnableExperiment    bool
	EnableBenchmark     bool
	EnableWarehouse     bool
	EnableGovernance    bool
	EnableEmbedding     bool
	EnableTenant        bool
	EnableModelServing  bool
	EnableComposition   bool
	EnableFreshness     bool
	EnableMigration     bool
	EnableSaaS          bool
	EnableGitOps        bool
	EnableCost          bool
	EnableCluster       bool
	EnableScheduler     bool
	EnableSLA           bool
	EnableUI            bool
	EnableDBT           bool

	// Optional dbt configuration
	DBTOptions *dbt.SyncOptions

	// Optional dependencies for extended handlers
	// Handlers are only registered if both Enable* flag is true AND dependency is provided
	DriftDetector  interface{ RegisterAlerter(interface{}) }
	LineageTracker interface {
		GetLineage(string) (interface{}, error)
	}
	SemanticSearch interface {
		Search(string, int) ([]interface{}, error)
	}
	WASMRuntime interface {
		Execute(string, []byte) ([]byte, error)
	}
	FederationClient interface {
		Query(string, interface{}) (interface{}, error)
	}
	QualityValidator interface {
		Validate(string, interface{}) error
	}
	AutogenGenerator interface {
		Generate(interface{}) (interface{}, error)
	}
	ExperimentEngine interface {
		GetExperiment(string) (interface{}, error)
	}
	GraphQLSchema interface {
		Execute(string, map[string]interface{}) (interface{}, error)
	}

	// Cluster components
	ClusterMembership   interface{ LocalNode() interface{} }
	ClusterRing         interface{ NodeCount() int }
	ClusterPartitionMap interface{ TotalPartitions() int }
	ClusterRebalancer   interface{ Stats() interface{} }
}

// DefaultMaxRequestSize is the default maximum request body size (1MB).
const DefaultMaxRequestSize = 1 << 20 // 1MB

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
	m *metrics.Metrics,
	cfg HTTPServerConfig,
) *HTTPServer {
	mux := http.NewServeMux()

	s := &HTTPServer{
		store:         store,
		aggregation:   agg,
		schema:        schema,
		metrics:       m,
		healthChecker: cfg.HealthChecker,
		mux:           mux,
	}

	s.registerRoutes()

	// Register vector routes if vector store is provided
	if cfg.VectorStore != nil {
		vectorHandler := NewVectorHandler(cfg.VectorStore, m)
		vectorHandler.RegisterRoutes(mux)
	}

	// Register optional handlers
	if cfg.EnableGroups {
		groupsHandler := NewGroupsHandler()
		groupsHandler.RegisterRoutes(mux)
	}

	if cfg.EnableBackfill {
		backfillHandler := NewBackfillHandler(store)
		backfillHandler.RegisterRoutes(mux)
	}

	if cfg.EnableStreaming {
		streamingHandler := NewStreamingHandler()
		streamingHandler.RegisterRoutes(mux)
	}

	if cfg.EnableCatalog {
		catalogHandler := NewCatalogHandler()
		catalogHandler.RegisterRoutes(mux)
	}

	if cfg.EnableAuth {
		authHandler := NewAuthHandler()
		authHandler.RegisterRoutes(mux)
	}

	// Handlers that require only the store
	if cfg.EnableML {
		mlHandler := NewMLHandler(store)
		mlHandler.RegisterRoutes(mux)
	}

	if cfg.EnableTransform {
		transformHandler := NewTransformHandler(store)
		transformHandler.RegisterRoutes(mux)
	}

	if cfg.EnableCache {
		cacheHandler := NewCacheHandler(store)
		cacheHandler.RegisterRoutes(mux)
	}

	if cfg.EnableConsistency {
		consistencyHandler := NewConsistencyHandler(store)
		consistencyHandler.RegisterRoutes(mux)
	}

	if cfg.EnableObservability {
		observabilityHandler := NewObservabilityHandler(store)
		observabilityHandler.RegisterRoutes(mux)
	}

	if cfg.EnableBenchmark {
		benchmarkHandler := NewBenchmarkHandler(store)
		benchmarkHandler.RegisterRoutes(mux)
	}

	if cfg.EnableTenant {
		tenantHandler := NewTenantHandler(4 * 1024 * 1024 * 1024) // 4GB default
		tenantHandler.RegisterRoutes(mux)
	}

	if cfg.EnableModelServing {
		modelServingHandler := NewModelServingHandler(store)
		modelServingHandler.RegisterRoutes(mux)
	}

	if cfg.EnableWarehouse {
		warehouseHandler := NewWarehouseHandler(WarehouseHandlerConfig{
			Store:  store,
			Schema: schema,
		})
		warehouseHandler.RegisterRoutes(mux)
	}

	if cfg.EnableGovernance {
		governanceHandler := NewGovernanceHandler(GovernanceHandlerConfig{})
		governanceHandler.RegisterRoutes(mux)
	}

	if cfg.EnableEmbedding {
		embeddingHandler := NewEmbeddingHandler(EmbeddingHandlerConfig{})
		embeddingHandler.RegisterRoutes(mux)
	}

	if cfg.EnableComposition {
		compositionEngine := composition.NewEngine(composition.EngineConfig{
			Store:          store,
			ExecutorConfig: composition.DefaultExecutorConfig(),
		})
		compositionHandler := NewCompositionHandler(compositionEngine)
		compositionHandler.RegisterRoutes(mux)
	}

	if cfg.EnableFreshness {
		freshnessManager := freshness.NewManager(freshness.DefaultManagerConfig())
		freshnessHandler := NewFreshnessHandler(freshnessManager)
		freshnessHandler.RegisterRoutes(mux)
	}

	if cfg.EnableMigration {
		migrationManager := migration.NewManager(migration.DefaultManagerConfig())
		migrationHandler := NewMigrationHandler(migrationManager)
		migrationHandler.RegisterRoutes(mux)
	}

	if cfg.EnableSaaS {
		planRegistry := saas.NewPlanRegistry()
		billingManager := saas.NewBillingManager(planRegistry)
		provisioningManager := saas.NewProvisioningManager(planRegistry, billingManager)
		saasHandler := NewSaaSHandler(planRegistry, billingManager, provisioningManager)
		saasHandler.RegisterRoutes(mux)
	}

	if cfg.EnableGitOps {
		schemaLoader := gitops.NewSchemaLoader(".")
		policyEngine := gitops.NewPolicyEngine()
		syncManager := gitops.NewSyncManager(schemaLoader, policyEngine, nil, ".gitops-state.json")
		gitopsHandler := NewGitOpsHandler(schemaLoader, policyEngine, syncManager)
		gitopsHandler.RegisterRoutes(mux)
	}

	if cfg.EnableCost {
		costTracker := cost.NewTracker("USD")
		budgetManager := cost.NewBudgetManager(costTracker)
		chargebackManager := cost.NewChargebackManager(costTracker)
		costHandler := NewCostHandler(costTracker, budgetManager, chargebackManager)
		costHandler.RegisterRoutes(mux)
	}

	if cfg.EnableCluster {
		// Create cluster handler with provided or default components
		var membership *cluster.MembershipManager
		var ring *cluster.HashRing
		var partitionMap *cluster.PartitionMap
		var rebalancer *cluster.Rebalancer

		// The handler works with nil components, returning 503 for unconfigured endpoints
		clusterHandler := NewClusterHandler(membership, ring, partitionMap, rebalancer)
		clusterHandler.RegisterRoutes(mux)
	}

	if cfg.EnableScheduler {
		scheduler := warehouse.NewCronScheduler(nil, slog.Default())
		schedulerHandler := NewSchedulerHandler(scheduler)
		schedulerHandler.RegisterRoutes(mux)
	}

	if cfg.EnableSLA {
		slaManager := sla.NewManager(nil, sla.DefaultManagerConfig())
		slaHandler := NewSLAHandler(slaManager)
		slaHandler.RegisterRoutes(mux)
	}

	// Handlers that don't require dependencies
	if cfg.EnableImpact {
		impactHandler := NewImpactHandler()
		impactHandler.RegisterRoutes(mux)
	}

	// Handlers that can work with default instances if enabled
	if cfg.EnableDrift {
		driftDetector := drift.NewDetector(drift.DefaultConfig())
		driftHandler := NewDriftHandler(driftDetector)
		driftHandler.RegisterRoutes(mux)
	}

	if cfg.EnableLineage {
		lineageTracker := lineage.NewTracker()
		lineageHandler := NewLineageHandler(lineageTracker)
		lineageHandler.RegisterRoutes(mux)
	}

	if cfg.EnableSemantic {
		// Use local embedder for semantic search (no external API needed)
		embedder := semantic.NewLocalEmbedder(128)
		semanticSearch := semantic.NewSearch(embedder, slog.Default())
		semanticHandler := NewSemanticHandler(semanticSearch)
		semanticHandler.RegisterRoutes(mux)
	}

	if cfg.EnableWASM {
		wasmRuntime := wasm.NewRuntime(wasm.DefaultConfig(), slog.Default())
		wasmHandler := NewWASMHandler(wasmRuntime)
		wasmHandler.RegisterRoutes(mux)
	}

	if cfg.EnableFederation {
		fed := federation.NewFederation(federation.DefaultConfig())
		federationHandler := NewFederationHandler(fed)
		federationHandler.RegisterRoutes(mux)
	}

	if cfg.EnableQuality {
		qualityValidator := quality.NewValidator()
		qualityHandler := NewQualityHandler(qualityValidator)
		qualityHandler.RegisterRoutes(mux)
	}

	if cfg.EnableGraphQL && store != nil && schema != nil {
		graphqlSchema, err := graphql.NewFeatureStoreSchema(store, schema)
		if err == nil {
			graphqlHandler := NewGraphQLHandler(graphqlSchema)
			graphqlHandler.RegisterRoutes(mux)
		}
	}

	if cfg.EnableAutogen {
		autogenGen := autogen.NewGenerator(autogen.DefaultConfig())
		autogenHandler := NewAutogenHandler(autogenGen)
		autogenHandler.RegisterRoutes(mux)
	}

	if cfg.EnableExperiment {
		experimentEngine := experiment.NewEngine()
		experimentHandler := NewExperimentHandler(experimentEngine)
		experimentHandler.RegisterRoutes(mux)
	}

	// Register UI handler for feature catalog
	if cfg.EnableUI {
		uiHandler, err := ui.NewHandler()
		if err == nil {
			uiHandler.RegisterRoutes(mux)
		}
	}

	// Register dbt integration handler
	if cfg.EnableDBT {
		dbtHandler := NewDBTHandler(cfg.DBTOptions)
		dbtHandler.RegisterRoutes(mux)
	}

	// Wrap handler with middleware chain
	var handler http.Handler = mux
	handler = requestIDMiddleware(handler)
	handler = compressionMiddleware(handler)

	// Add request size limit middleware
	maxSize := cfg.MaxRequestSize
	if maxSize == 0 {
		maxSize = DefaultMaxRequestSize
	}
	handler = maxRequestSizeMiddleware(maxSize)(handler)

	// Add CORS middleware if enabled
	if cfg.EnableCORS {
		handler = corsMiddleware(cfg.CORS)(handler)
	}

	// Add security headers middleware
	tlsEnabled := cfg.TLS != nil && cfg.TLS.Enabled
	handler = securityHeadersMiddleware(tlsEnabled)(handler)

	// Add panic recovery middleware (outermost to catch all panics)
	handler = panicRecoveryMiddleware(handler)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	// Configure TLS if enabled
	s.tlsConfig = cfg.TLS
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig, err := cfg.TLS.BuildTLSConfig()
		if err == nil && tlsConfig != nil {
			s.server.TLSConfig = tlsConfig
		}
	}

	return s
}

func (s *HTTPServer) registerRoutes() {
	// Feature routes
	s.mux.HandleFunc("GET /v1/features", s.handleGetFeatures)
	s.mux.HandleFunc("POST /v1/features", s.handlePutFeatures)
	s.mux.HandleFunc("POST /v1/features/batch", s.handleGetFeaturesBatch)
	s.mux.HandleFunc("GET /v1/features/history", s.handleGetFeaturesAsOf)

	// Schema routes
	s.mux.HandleFunc("GET /v1/schema/groups", s.handleListGroups)
	s.mux.HandleFunc("GET /v1/schema/groups/{name}", s.handleGetGroup)
	s.mux.HandleFunc("POST /v1/schema/groups", s.handleCreateGroup)

	// Health routes
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)
	s.mux.HandleFunc("GET /live", s.handleLive)
}

// Start starts the HTTP server.
// If TLS is enabled, it starts with HTTPS using the configured certificate.
func (s *HTTPServer) Start() error {
	if s.tlsConfig != nil && s.tlsConfig.Enabled {
		return s.server.ListenAndServeTLS(s.tlsConfig.CertFile, s.tlsConfig.KeyFile)
	}
	return s.server.ListenAndServe()
}

// IsTLSEnabled returns true if the server is configured to use TLS.
func (s *HTTPServer) IsTLSEnabled() bool {
	return s.tlsConfig != nil && s.tlsConfig.Enabled
}

// GetTLSConfig returns the server's TLS configuration for testing.
func (s *HTTPServer) GetTLSConfig() *tls.Config {
	return s.server.TLSConfig
}

// Stop gracefully stops the server.
func (s *HTTPServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleGetFeatures handles GET /v1/features
func (s *HTTPServer) handleGetFeatures(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordHTTPLatency("GET", "/v1/features", time.Since(start))
		}
	}()

	entityKey := r.URL.Query().Get("entity")
	if entityKey == "" {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "entity query parameter required")
		return
	}

	featureNames := r.URL.Query()["feature"]
	if len(featureNames) == 0 {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "at least one feature query parameter required")
		return
	}

	features, err := s.getEntityFeatures(entityKey, featureNames)
	if err != nil {
		s.writeErrorFromErr(w, err)
		return
	}

	s.writeAPIResponse(w, r, http.StatusOK, domain.GetFeaturesResponse{
		Entities: map[string]*domain.EntityFeatures{
			entityKey: features,
		},
	})
}

// handleGetFeaturesBatch handles POST /v1/features/batch
func (s *HTTPServer) handleGetFeaturesBatch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordHTTPLatency("POST", "/v1/features/batch", time.Since(start))
		}
	}()

	var req domain.GetFeaturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeBadRequest, "invalid request body")
		return
	}

	if len(req.Entities) == 0 {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "entities required")
		return
	}
	if len(req.Features) == 0 {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "features required")
		return
	}

	result := domain.GetFeaturesResponse{
		Entities: make(map[string]*domain.EntityFeatures),
	}

	for _, entityKey := range req.Entities {
		features, err := s.getEntityFeatures(entityKey, req.Features)
		if err != nil {
			if domain.IsNotFound(err) {
				continue
			}
			s.writeErrorFromErr(w, err)
			return
		}
		result.Entities[entityKey] = features
	}

	s.writeAPIResponse(w, r, http.StatusOK, result)
}

// handleGetFeaturesAsOf handles GET /v1/features/history
func (s *HTTPServer) handleGetFeaturesAsOf(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordHTTPLatency("GET", "/v1/features/history", time.Since(start))
		}
	}()

	entityKey := r.URL.Query().Get("entity")
	if entityKey == "" {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "entity query parameter required")
		return
	}

	asOfStr := r.URL.Query().Get("as_of")
	if asOfStr == "" {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "as_of query parameter required")
		return
	}

	asOf, err := time.Parse(time.RFC3339, asOfStr)
	if err != nil {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "as_of must be in RFC3339 format")
		return
	}

	featureNames := r.URL.Query()["feature"]
	if len(featureNames) == 0 {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "at least one feature query parameter required")
		return
	}

	features, err := s.store.GetAsOf(entityKey, featureNames, asOf)
	if err != nil {
		s.writeErrorFromErr(w, err)
		return
	}

	entityFeatures := &domain.EntityFeatures{
		Features: make(map[string]*domain.Feature),
	}
	for name, val := range features {
		entityFeatures.Features[name] = &domain.Feature{
			Value:     val.Value,
			Timestamp: val.Timestamp,
		}
	}

	s.writeAPIResponse(w, r, http.StatusOK, domain.GetFeaturesResponse{
		Entities: map[string]*domain.EntityFeatures{
			entityKey: entityFeatures,
		},
	})
}

// handlePutFeatures handles POST /v1/features
func (s *HTTPServer) handlePutFeatures(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordHTTPLatency("POST", "/v1/features", time.Since(start))
		}
	}()

	var req domain.FeatureUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.EntityKey == "" {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "entity_key required")
		return
	}

	timestamp := req.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().UnixNano()
	}

	features := make(map[string]*domain.FeatureValue)
	for name, val := range req.Features {
		features[name] = &domain.FeatureValue{
			Value:     val,
			Timestamp: timestamp,
			Version:   req.Version,
		}

		// Update aggregations
		if s.aggregation.GetSpec(name) != nil {
			if floatVal, ok := val.(float64); ok {
				s.aggregation.Update(req.EntityKey, name, floatVal, time.Now())
			}
		}
	}

	if err := s.store.Put(req.EntityKey, features); err != nil {
		s.writeErrorFromErr(w, err)
		return
	}

	s.writeAPIResponse(w, r, http.StatusCreated, map[string]bool{"success": true})
}

// handleListGroups handles GET /v1/schema/groups
func (s *HTTPServer) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups := s.schema.ListGroups()
	s.writeAPIResponse(w, r, http.StatusOK, groups)
}

// handleGetGroup handles GET /v1/schema/groups/{name}
func (s *HTTPServer) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeValidationFailed, "group name required")
		return
	}

	group, err := s.schema.GetGroup(name)
	if err != nil {
		s.writeErrorFromErr(w, err)
		return
	}

	s.writeAPIResponse(w, r, http.StatusOK, group)
}

// handleCreateGroup handles POST /v1/schema/groups
func (s *HTTPServer) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var group domain.FeatureGroup
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		s.writeErrorWithCode(w, http.StatusBadRequest, domain.ErrCodeBadRequest, "invalid request body")
		return
	}

	if err := s.schema.RegisterGroup(&group); err != nil {
		s.writeErrorFromErr(w, err)
		return
	}

	// Register aggregations
	for _, feature := range group.Features {
		if feature.Aggregation != nil {
			s.aggregation.RegisterAggregation(feature.Name, feature.Aggregation)
		}
	}

	s.writeAPIResponse(w, r, http.StatusCreated, group)
}

// handleHealth handles GET /health - deep health check
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.healthChecker != nil {
		result := s.healthChecker.Check(r.Context())
		status := http.StatusOK
		if result.Status == HealthStatusDegraded {
			status = http.StatusOK // Still return 200 for degraded
		} else if result.Status == HealthStatusUnhealthy {
			status = http.StatusServiceUnavailable
		}
		s.writeJSON(w, status, result)
		return
	}
	// Fallback to simple health check
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// handleReady handles GET /ready - readiness check for k8s
func (s *HTTPServer) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.healthChecker != nil {
		if !s.healthChecker.ReadinessCheck() {
			s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not_ready",
			})
			return
		}
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleLive handles GET /live - liveness check for k8s
func (s *HTTPServer) handleLive(w http.ResponseWriter, r *http.Request) {
	if s.healthChecker != nil {
		if !s.healthChecker.LivenessCheck() {
			s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unhealthy",
			})
			return
		}
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

// getEntityFeatures retrieves features for an entity.
func (s *HTTPServer) getEntityFeatures(entityKey string, featureNames []string) (*domain.EntityFeatures, error) {
	features, err := s.store.Get(entityKey, featureNames)
	if err != nil && !domain.IsNotFound(err) {
		return nil, err
	}

	result := &domain.EntityFeatures{
		Features: make(map[string]*domain.Feature),
	}

	now := time.Now().UnixNano()
	for name, val := range features {
		result.Features[name] = &domain.Feature{
			Value:     val.Value,
			Timestamp: val.Timestamp,
		}

		// Record feature freshness metrics
		if s.metrics != nil {
			age := time.Duration(now - val.Timestamp)
			s.metrics.SetFeatureFreshness(name, age)
			s.metrics.RecordFeatureRequest(name)
		}
	}

	// Compute aggregations for missing features
	for _, name := range featureNames {
		if _, ok := result.Features[name]; ok {
			continue
		}

		if spec := s.aggregation.GetSpec(name); spec != nil {
			val, err := s.aggregation.ComputeWithSpec(entityKey, name)
			if err == nil {
				result.Features[name] = &domain.Feature{
					Value:     val,
					Timestamp: time.Now().UnixNano(),
				}
			}
		}
	}

	return result, nil
}

func (s *HTTPServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.FromContext(context.Background(), nil).Error("failed to encode JSON response", "error", err)
	}
}

// writeAPIResponse writes a standardized API response with optional request ID.
func (s *HTTPServer) writeAPIResponse(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	resp := domain.NewSuccessResponse(data)

	// Add request ID from context or header
	if requestID := w.Header().Get("X-Request-ID"); requestID != "" {
		resp.WithRequestID(requestID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logging.FromContext(r.Context(), nil).Error("failed to encode JSON response", "error", err)
	}
}

func (s *HTTPServer) writeError(w http.ResponseWriter, status int, message string) {
	s.writeErrorWithCode(w, status, statusToErrorCode(status), message)
}

// writeErrorWithCode writes a standardized error response with error code and request ID.
func (s *HTTPServer) writeErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	resp := domain.NewErrorResponse(code, message)

	// Add request ID from response header (set by middleware)
	if requestID := w.Header().Get("X-Request-ID"); requestID != "" {
		resp.WithRequestID(requestID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logging.FromContext(context.Background(), nil).Error("failed to encode JSON error response", "error", err)
	}
}

// writeErrorFromErr writes an error response, deriving the error code from the error type.
func (s *HTTPServer) writeErrorFromErr(w http.ResponseWriter, err error) {
	code := domain.ErrorToCode(err)
	status := errorCodeToStatus(code)
	s.writeErrorWithCode(w, status, code, err.Error())
}

// statusToErrorCode maps HTTP status codes to error codes.
func statusToErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return domain.ErrCodeBadRequest
	case http.StatusUnauthorized:
		return domain.ErrCodeUnauthorized
	case http.StatusForbidden:
		return domain.ErrCodeForbidden
	case http.StatusNotFound:
		return domain.ErrCodeNotFound
	case http.StatusConflict:
		return domain.ErrCodeConflict
	case http.StatusTooManyRequests:
		return domain.ErrCodeRateLimited
	case http.StatusRequestEntityTooLarge:
		return domain.ErrCodeRequestTooLarge
	case http.StatusServiceUnavailable:
		return domain.ErrCodeServiceUnavailable
	case http.StatusGatewayTimeout:
		return domain.ErrCodeTimeout
	default:
		return domain.ErrCodeInternal
	}
}

// errorCodeToStatus maps error codes to HTTP status codes.
func errorCodeToStatus(code string) int {
	switch code {
	case domain.ErrCodeBadRequest, domain.ErrCodeValidationFailed:
		return http.StatusBadRequest
	case domain.ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case domain.ErrCodeForbidden:
		return http.StatusForbidden
	case domain.ErrCodeNotFound:
		return http.StatusNotFound
	case domain.ErrCodeConflict:
		return http.StatusConflict
	case domain.ErrCodeRateLimited:
		return http.StatusTooManyRequests
	case domain.ErrCodeRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case domain.ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case domain.ErrCodeStorageFull:
		return http.StatusInsufficientStorage
	case domain.ErrCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// ServeHTTP implements http.Handler for testing.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Shutdown gracefully shuts down the server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

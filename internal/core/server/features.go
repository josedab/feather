// Package server provides the HTTP and gRPC serving layer for Feather.
//
// # Handler Registration System
//
// The server uses a pluggable handler architecture (see ADR-0008) to support
// optional feature modules. Each feature area (drift, lineage, marketplace, etc.)
// implements the [FeatureHandler] interface and registers a factory function in
// [featureRegistry] during init().
//
// To enable a handler, add its name to the EnabledFeatures map in
// [HTTPServerFeatureConfig]. Only enabled handlers are instantiated and have
// their routes registered on the ServeMux.
//
// To add a new handler:
//  1. Create a struct implementing [FeatureHandler] (with a RegisterRoutes method).
//  2. Register a factory in the init() function of this file.
//  3. Enable it via the EnabledFeatures map in cmd/feather/main.go.
//  4. Add it to docs/package-guide.md.
package server

import (
"context"
"log/slog"
"net/http"

"github.com/feather-store/feather/internal/core/aggregation"
"github.com/feather-store/feather/internal/core/metrics"
"github.com/feather-store/feather/internal/core/storage"
"github.com/feather-store/feather/internal/extensions/autogen"
"github.com/feather-store/feather/internal/extensions/composition"
"github.com/feather-store/feather/internal/extensions/computegraph"
"github.com/feather-store/feather/internal/extensions/drift"
"github.com/feather-store/feather/internal/extensions/experiment"
"github.com/feather-store/feather/internal/extensions/freshness"
"github.com/feather-store/feather/internal/extensions/graphql"
"github.com/feather-store/feather/internal/extensions/lineage"
"github.com/feather-store/feather/internal/extensions/llmfeature"
"github.com/feather-store/feather/internal/extensions/llmgateway"
"github.com/feather-store/feather/internal/extensions/materialization"
"github.com/feather-store/feather/internal/extensions/rag"
"github.com/feather-store/feather/internal/extensions/semantic"
"github.com/feather-store/feather/internal/extensions/skewdetect"
"github.com/feather-store/feather/internal/extensions/streamdsl"
"github.com/feather-store/feather/internal/extensions/timetravel"
"github.com/feather-store/feather/internal/extensions/versioning"
"github.com/feather-store/feather/internal/extensions/wasm"
"github.com/feather-store/feather/internal/integrations/streamsql"
"github.com/feather-store/feather/internal/integrations/warehouse"
"github.com/feather-store/feather/internal/platform/autoscaler"
"github.com/feather-store/feather/internal/platform/cloud"
"github.com/feather-store/feather/internal/platform/consensus"
"github.com/feather-store/feather/internal/platform/contract"
"github.com/feather-store/feather/internal/platform/controlplane"
"github.com/feather-store/feather/internal/platform/cost"
"github.com/feather-store/feather/internal/platform/federation"
"github.com/feather-store/feather/internal/platform/finops"
"github.com/feather-store/feather/internal/platform/gitops"
"github.com/feather-store/feather/internal/platform/migration"
"github.com/feather-store/feather/internal/platform/monitoring"
"github.com/feather-store/feather/internal/platform/multiregion"
"github.com/feather-store/feather/internal/platform/parity"
"github.com/feather-store/feather/internal/platform/plugin"
"github.com/feather-store/feather/internal/platform/pushdown"
"github.com/feather-store/feather/internal/platform/quality"
"github.com/feather-store/feather/internal/platform/replication"
"github.com/feather-store/feather/internal/platform/saas"
"github.com/feather-store/feather/internal/platform/sla"
"github.com/feather-store/feather/internal/platform/validation"
"github.com/feather-store/feather/internal/tools/benchsuite"
"github.com/feather-store/feather/internal/tools/catalog"
"github.com/feather-store/feather/internal/tools/compute"
"github.com/feather-store/feather/internal/tools/playground"
"github.com/feather-store/feather/internal/tools/ui"
)

// FeatureHandler registers routes on a ServeMux.
type FeatureHandler interface {
RegisterRoutes(mux *http.ServeMux)
}

// handlerDeps provides dependencies to handler factories.
type handlerDeps struct {
Ctx         context.Context
Store       *storage.Store
Aggregation *aggregation.Engine
Schema      *storage.Registry
Metrics     *metrics.Metrics
Config      HTTPServerConfig
}

// handlerFactory creates a FeatureHandler from dependencies.
// Returns nil if the handler cannot be created.
type handlerFactory func(deps *handlerDeps) FeatureHandler

// featureRegistry maps feature names to factory functions.
var featureRegistry = map[string]handlerFactory{}

func init() {
// --- Core handlers ---
featureRegistry["groups"] = func(deps *handlerDeps) FeatureHandler {
return NewGroupsHandler()
}
featureRegistry["backfill"] = func(deps *handlerDeps) FeatureHandler {
return NewBackfillHandler(deps.Store)
}
featureRegistry["streaming"] = func(deps *handlerDeps) FeatureHandler {
return NewStreamingHandler(deps.Ctx)
}
featureRegistry["catalog"] = func(deps *handlerDeps) FeatureHandler {
return NewCatalogHandler()
}
featureRegistry["auth"] = func(deps *handlerDeps) FeatureHandler {
return NewAuthHandler()
}
featureRegistry["ml"] = func(deps *handlerDeps) FeatureHandler {
return NewMLHandler(deps.Store)
}
featureRegistry["transform"] = func(deps *handlerDeps) FeatureHandler {
return NewTransformHandler(deps.Store)
}
featureRegistry["cache"] = func(deps *handlerDeps) FeatureHandler {
return NewCacheHandler(deps.Store)
}
featureRegistry["consistency"] = func(deps *handlerDeps) FeatureHandler {
return NewConsistencyHandler(deps.Store)
}
featureRegistry["observability"] = func(deps *handlerDeps) FeatureHandler {
return NewObservabilityHandler(deps.Store)
}
featureRegistry["benchmark"] = func(deps *handlerDeps) FeatureHandler {
return NewBenchmarkHandler(deps.Store)
}
featureRegistry["impact"] = func(deps *handlerDeps) FeatureHandler {
return NewImpactHandler()
}
featureRegistry["model_serving"] = func(deps *handlerDeps) FeatureHandler {
return NewModelServingHandler(deps.Store)
}

// --- Handlers with special dependency setup ---
featureRegistry["tenant"] = func(deps *handlerDeps) FeatureHandler {
maxBytes := deps.Config.Core.TenantMaxBytes
if maxBytes == 0 {
maxBytes = 4 * 1024 * 1024 * 1024
}
return NewTenantHandler(maxBytes)
}
featureRegistry["warehouse"] = func(deps *handlerDeps) FeatureHandler {
return NewWarehouseHandler(WarehouseHandlerConfig{
Store:  deps.Store,
Schema: deps.Schema,
})
}
featureRegistry["governance"] = func(deps *handlerDeps) FeatureHandler {
return NewGovernanceHandler(GovernanceHandlerConfig{})
}
featureRegistry["embedding"] = func(deps *handlerDeps) FeatureHandler {
return NewEmbeddingHandler(EmbeddingHandlerConfig{})
}
featureRegistry["composition"] = func(deps *handlerDeps) FeatureHandler {
engine := composition.NewEngine(composition.EngineConfig{
Store:          deps.Store,
ExecutorConfig: composition.DefaultExecutorConfig(),
})
return NewCompositionHandler(engine)
}
featureRegistry["freshness"] = func(deps *handlerDeps) FeatureHandler {
return NewFreshnessHandler(freshness.NewManager(freshness.DefaultManagerConfig()))
}
featureRegistry["migration"] = func(deps *handlerDeps) FeatureHandler {
return NewMigrationHandler(migration.NewManager(migration.DefaultManagerConfig()))
}
featureRegistry["saas"] = func(deps *handlerDeps) FeatureHandler {
reg := saas.NewPlanRegistry()
billing := saas.NewBillingManager(reg)
return NewSaaSHandler(reg, billing, saas.NewProvisioningManager(reg, billing))
}
featureRegistry["gitops"] = func(deps *handlerDeps) FeatureHandler {
loader := gitops.NewSchemaLoader(".")
policy := gitops.NewPolicyEngine()
return NewGitOpsHandler(loader, policy, gitops.NewSyncManager(loader, policy, nil, ".gitops-state.json"))
}
featureRegistry["cost"] = func(deps *handlerDeps) FeatureHandler {
tracker := cost.NewTracker("USD")
return NewCostHandler(tracker, cost.NewBudgetManager(tracker), cost.NewChargebackManager(tracker))
}
featureRegistry["cluster"] = func(deps *handlerDeps) FeatureHandler {
	// Cluster handler requires external cluster configuration.
	// Return nil to skip registration when deps are not available.
	return nil
}
featureRegistry["scheduler"] = func(deps *handlerDeps) FeatureHandler {
return NewSchedulerHandler(warehouse.NewCronScheduler(nil, slog.Default()))
}
featureRegistry["sla"] = func(deps *handlerDeps) FeatureHandler {
return NewSLAHandler(sla.NewManager(nil, sla.DefaultManagerConfig()))
}
featureRegistry["drift"] = func(deps *handlerDeps) FeatureHandler {
return NewDriftHandler(drift.NewDetector(drift.DefaultConfig()))
}
featureRegistry["lineage"] = func(deps *handlerDeps) FeatureHandler {
return NewLineageHandler(lineage.NewTracker())
}
featureRegistry["semantic"] = func(deps *handlerDeps) FeatureHandler {
return NewSemanticHandler(semantic.NewSearch(semantic.NewLocalEmbedder(128), slog.Default()))
}
featureRegistry["wasm"] = func(deps *handlerDeps) FeatureHandler {
return NewWASMHandler(wasm.NewRuntime(wasm.DefaultConfig(), slog.Default()))
}
featureRegistry["federation"] = func(deps *handlerDeps) FeatureHandler {
return NewFederationHandler(federation.NewFederation(federation.DefaultConfig()))
}
featureRegistry["quality"] = func(deps *handlerDeps) FeatureHandler {
return NewQualityHandler(quality.NewValidator())
}
featureRegistry["graphql"] = func(deps *handlerDeps) FeatureHandler {
if deps.Store == nil || deps.Schema == nil {
return nil
}
s, err := graphql.NewFeatureStoreSchema(deps.Store, deps.Schema)
if err != nil {
return nil
}
return NewGraphQLHandler(s)
}
featureRegistry["autogen"] = func(deps *handlerDeps) FeatureHandler {
return NewAutogenHandler(autogen.NewGenerator(autogen.DefaultConfig()))
}
featureRegistry["experiment"] = func(deps *handlerDeps) FeatureHandler {
return NewExperimentHandler(experiment.NewEngine())
}
featureRegistry["ui"] = func(deps *handlerDeps) FeatureHandler {
h, err := ui.NewHandler()
if err != nil {
return nil
}
return h
}
featureRegistry["dbt"] = func(deps *handlerDeps) FeatureHandler {
return NewDBTHandler(deps.Config.Dependencies.DBTOptions)
}
featureRegistry["compute"] = func(deps *handlerDeps) FeatureHandler {
return NewComputeHandler(compute.NewComputeEngine(compute.DefaultComputeConfig()))
}
featureRegistry["consensus"] = func(deps *handlerDeps) FeatureHandler {
node := consensus.NewRaftNode(consensus.DefaultRaftConfig(), nil)
return NewConsensusHandler(node, consensus.NewShardManager(16, node))
}
featureRegistry["stream_sql"] = func(deps *handlerDeps) FeatureHandler {
return NewStreamSQLHandler(streamsql.NewEngine(streamsql.DefaultEngineConfig()))
}
featureRegistry["control_plane"] = func(deps *handlerDeps) FeatureHandler {
return NewControlPlaneHandler(controlplane.NewManager(controlplane.DefaultManagerConfig()))
}
featureRegistry["rag"] = func(deps *handlerDeps) FeatureHandler {
return NewRAGHandler(rag.NewPipeline(rag.DefaultPipelineConfig()))
}
featureRegistry["plugin"] = func(deps *handlerDeps) FeatureHandler {
return NewPluginHandler(plugin.NewRegistry(plugin.DefaultRegistryConfig()))
}
featureRegistry["versioning"] = func(deps *handlerDeps) FeatureHandler {
return NewVersioningHandler(versioning.NewVersionStore())
}
featureRegistry["validation"] = func(deps *handlerDeps) FeatureHandler {
return NewValidationHandler(validation.NewValidator(validation.DefaultValidatorConfig()))
}
featureRegistry["dashboard_v2"] = func(deps *handlerDeps) FeatureHandler {
return NewDashboardHandler(deps.Store, deps.Metrics)
}

featureRegistry["sharding"] = func(deps *handlerDeps) FeatureHandler {
return nil
}

featureRegistry["marketplace"] = func(deps *handlerDeps) FeatureHandler {
return NewMarketplaceHandler()
}
featureRegistry["cloud_service"] = func(deps *handlerDeps) FeatureHandler {
return NewCloudServiceHandler()
}
featureRegistry["featherql"] = func(deps *handlerDeps) FeatureHandler {
return NewFeatherQLHandler()
}
featureRegistry["llm_cache"] = func(deps *handlerDeps) FeatureHandler {
return NewLLMCacheHandler()
}
featureRegistry["autofe"] = func(deps *handlerDeps) FeatureHandler {
return NewAutoFEHandler()
}
featureRegistry["geo_routing"] = func(deps *handlerDeps) FeatureHandler {
return NewGeoRoutingHandler()
}
featureRegistry["ab_rollout"] = func(deps *handlerDeps) FeatureHandler {
return NewABRolloutHandler()
}
featureRegistry["edge_runtime"] = func(deps *handlerDeps) FeatureHandler {
return NewEdgeRuntimeHandler()
}
featureRegistry["contracts"] = func(deps *handlerDeps) FeatureHandler {
return NewContractHandler(contract.NewManager(contract.DefaultManagerConfig(), nil))
}
featureRegistry["materialization"] = func(deps *handlerDeps) FeatureHandler {
return NewMaterializationHandler(materialization.NewEngine(materialization.DefaultEngineConfig()))
}
featureRegistry["playground"] = func(deps *handlerDeps) FeatureHandler {
return NewPlaygroundHandler(playground.NewService(nil))
}
featureRegistry["replication"] = func(deps *handlerDeps) FeatureHandler {
return NewReplicationHandler(replication.NewManager(replication.DefaultManagerConfig()))
}
featureRegistry["pushdown"] = func(deps *handlerDeps) FeatureHandler {
return NewPushdownHandler(pushdown.NewEvaluator())
}
featureRegistry["llm_features"] = func(deps *handlerDeps) FeatureHandler {
return NewLLMFeatureHandler(llmfeature.NewStore(llmfeature.DefaultStoreConfig()))
}
featureRegistry["finops"] = func(deps *handlerDeps) FeatureHandler {
return NewFinOpsHandler(finops.NewManager(finops.DefaultManagerConfig()))
}
featureRegistry["parity"] = func(deps *handlerDeps) FeatureHandler {
return NewParityHandler(parity.NewChecker(parity.DefaultConfig()))
}
featureRegistry["monitoring"] = func(deps *handlerDeps) FeatureHandler {
return NewMonitoringHandler(monitoring.NewManager(monitoring.DefaultManagerConfig()))
}
featureRegistry["time_travel"] = func(deps *handlerDeps) FeatureHandler {
return NewTimeTravelHandler(timetravel.NewDebugger(timetravel.DefaultDebuggerConfig()))
}
featureRegistry["catalog_ui"] = func(deps *handlerDeps) FeatureHandler {
return NewCatalogUIHandler(catalog.NewService(catalog.DefaultConfig()))
}
featureRegistry["feather_cloud"] = func(deps *handlerDeps) FeatureHandler {
return NewCloudHandler(cloud.NewControlPlane(cloud.DefaultConfig()))
}
featureRegistry["stream_dsl"] = func(deps *handlerDeps) FeatureHandler {
return NewStreamDSLHandler(streamdsl.NewPipelineManager(streamdsl.DefaultCompilerConfig()))
}
featureRegistry["llm_gateway"] = func(deps *handlerDeps) FeatureHandler {
return NewLLMGatewayHandler(llmgateway.NewGateway(llmgateway.DefaultGatewayConfig()))
}
featureRegistry["skew_detect"] = func(deps *handlerDeps) FeatureHandler {
return NewSkewDetectHandler(skewdetect.NewDetector(skewdetect.DefaultDetectorConfig()))
}
featureRegistry["compute_graph"] = func(deps *handlerDeps) FeatureHandler {
return NewComputeGraphHandler(computegraph.NewEngine(computegraph.DefaultEngineConfig()))
}
featureRegistry["k8s_autoscaler"] = func(deps *handlerDeps) FeatureHandler {
return NewAutoscalerHandler(autoscaler.NewAutoscaler(autoscaler.DefaultConfig()))
}
featureRegistry["multi_region"] = func(deps *handlerDeps) FeatureHandler {
return NewMultiRegionHandler(multiregion.NewFederation(multiregion.DefaultFederationConfig()))
}
featureRegistry["bench_suite"] = func(deps *handlerDeps) FeatureHandler {
return NewBenchSuiteHandler(benchsuite.NewSuite(benchsuite.DefaultSuiteConfig()))
}
}

// RegisteredFeatures returns all available feature names.
func RegisteredFeatures() []string {
names := make([]string, 0, len(featureRegistry))
for name := range featureRegistry {
names = append(names, name)
}
return names
}

// registerEnabledFeatures creates and registers all enabled feature handlers.
func registerEnabledFeatures(mux *http.ServeMux, enabled map[string]bool, deps *handlerDeps) {
for name, factory := range featureRegistry {
if !enabled[name] {
continue
}
handler := factory(deps)
if handler != nil {
handler.RegisterRoutes(mux)
}
}
}

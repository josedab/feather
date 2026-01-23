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
"github.com/feather-store/feather/internal/extensions/consistencyvalidator"
"github.com/feather-store/feather/internal/extensions/drift"
"github.com/feather-store/feather/internal/extensions/experiment"
"github.com/feather-store/feather/internal/extensions/freshness"
"github.com/feather-store/feather/internal/extensions/ftl"
"github.com/feather-store/feather/internal/extensions/graphql"
"github.com/feather-store/feather/internal/extensions/lineage"
"github.com/feather-store/feather/internal/extensions/llmfeature"
"github.com/feather-store/feather/internal/extensions/marketplace"
"github.com/feather-store/feather/internal/extensions/llmgateway"
"github.com/feather-store/feather/internal/extensions/materialization"
"github.com/feather-store/feather/internal/extensions/mobilesync"
"github.com/feather-store/feather/internal/extensions/qualityscore"
"github.com/feather-store/feather/internal/extensions/rag"
"github.com/feather-store/feather/internal/extensions/semantic"
"github.com/feather-store/feather/internal/extensions/skewdetect"
"github.com/feather-store/feather/internal/extensions/streamdsl"
"github.com/feather-store/feather/internal/extensions/timetravel"
"github.com/feather-store/feather/internal/extensions/versioning"
"github.com/feather-store/feather/internal/extensions/wasm"
"github.com/feather-store/feather/internal/extensions/streamcompute"
"github.com/feather-store/feather/internal/extensions/sdkcodegen"
"github.com/feather-store/feather/internal/extensions/schemaevolution"
"github.com/feather-store/feather/internal/extensions/saascontrol"
"github.com/feather-store/feather/internal/extensions/promptstore"
"github.com/feather-store/feather/internal/extensions/featherqlv2"
"github.com/feather-store/feather/internal/extensions/featuredashboard"
"github.com/feather-store/feather/internal/extensions/feastcompat"
"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
"github.com/feather-store/feather/internal/extensions/abfeatures"
"github.com/feather-store/feather/internal/extensions/adaptivecache"
"github.com/feather-store/feather/internal/extensions/backpressure"
"github.com/feather-store/feather/internal/extensions/benchpub"
"github.com/feather-store/feather/internal/extensions/contracttest"
"github.com/feather-store/feather/internal/extensions/federateddiscovery"
"github.com/feather-store/feather/internal/extensions/gitopsdefs"
"github.com/feather-store/feather/internal/extensions/lineagegraph"
"github.com/feather-store/feather/internal/extensions/offlinestore"
"github.com/feather-store/feather/internal/extensions/wasmudf"
"github.com/feather-store/feather/internal/extensions/anomalydetect"
"github.com/feather-store/feather/internal/extensions/apigateway"
"github.com/feather-store/feather/internal/extensions/auditlog"
"github.com/feather-store/feather/internal/extensions/cloudstorage"
"github.com/feather-store/feather/internal/extensions/feathercli"
"github.com/feather-store/feather/internal/extensions/importancescoring"
"github.com/feather-store/feather/internal/extensions/incrmat"
"github.com/feather-store/feather/internal/extensions/openapisync"
"github.com/feather-store/feather/internal/extensions/terraformprovider"
"github.com/feather-store/feather/internal/extensions/webhooks"
"github.com/feather-store/feather/internal/integrations/airflow"
"github.com/feather-store/feather/internal/integrations/kubeflow"
"github.com/feather-store/feather/internal/integrations/mlflow"
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
"github.com/feather-store/feather/internal/tools/dashboard"
"github.com/feather-store/feather/internal/tools/compute"
"github.com/feather-store/feather/internal/tools/playground"
"github.com/feather-store/feather/internal/tools/ui"
)

// FeatureHandler registers routes on a ServeMux.
type FeatureHandler interface {
RegisterRoutes(mux *http.ServeMux)
}

// Maturity indicates the stability level of a handler.
type Maturity string

const (
// MaturityStable means production-ready, well-tested, semver-protected.
MaturityStable Maturity = "stable"
// MaturityBeta means functional and tested, API may change between minor releases.
MaturityBeta Maturity = "beta"
// MaturityExperimental means working implementation, may be incomplete or change significantly.
MaturityExperimental Maturity = "experimental"
)

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

// HandlerSpec describes a registered handler and its maturity level.
type HandlerSpec struct {
Name     string
Maturity Maturity
Factory  handlerFactory
}

// featureRegistry maps feature names to factory functions.
var featureRegistry = map[string]handlerFactory{}

// handlerSpecs stores maturity metadata for every registered handler.
// Query with RegisteredHandlerSpecs() to inspect maturity levels.
var handlerSpecs []HandlerSpec

// registerHandler registers a handler with its maturity level.
func registerHandler(name string, maturity Maturity, factory handlerFactory) {
featureRegistry[name] = factory
handlerSpecs = append(handlerSpecs, HandlerSpec{Name: name, Maturity: maturity, Factory: factory})
}

func init() {
// ╔══════════════════════════════════════════════════════════════════╗
// ║  STABLE — Production-ready, well-tested, breaking changes      ║
// ║  follow semver. Safe for all deployments.                       ║
// ╚══════════════════════════════════════════════════════════════════╝
registerHandler("groups", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewGroupsHandler()
})
registerHandler("backfill", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewBackfillHandler(deps.Store)
})
registerHandler("streaming", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewStreamingHandler(deps.Ctx)
})
registerHandler("catalog", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewCatalogHandler()
})
registerHandler("auth", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewAuthHandler()
})
registerHandler("ml", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewMLHandler(deps.Store)
})
registerHandler("transform", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewTransformHandler(deps.Store)
})
registerHandler("cache", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewCacheHandler(deps.Store)
})
registerHandler("consistency", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewConsistencyHandler(deps.Store)
})
registerHandler("observability", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewObservabilityHandler(deps.Store)
})
registerHandler("benchmark", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewBenchmarkHandler(deps.Store)
})
registerHandler("impact", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewImpactHandler()
})
registerHandler("model_serving", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewModelServingHandler(deps.Store)
})

// ╔══════════════════════════════════════════════════════════════════╗
// ║  BETA — Functional and tested, API may change between minor    ║
// ║  releases. Suitable for staging and non-critical production.    ║
// ╚══════════════════════════════════════════════════════════════════╝

registerHandler("tenant", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
maxBytes := deps.Config.Core.TenantMaxBytes
if maxBytes == 0 {
maxBytes = 4 * 1024 * 1024 * 1024
}
return NewTenantHandler(maxBytes)
})
registerHandler("warehouse", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewWarehouseHandler(WarehouseHandlerConfig{
Store:  deps.Store,
Schema: deps.Schema,
})
})
registerHandler("governance", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewGovernanceHandler(GovernanceHandlerConfig{})
})
registerHandler("embedding", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewEmbeddingHandler(EmbeddingHandlerConfig{})
})
registerHandler("composition", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
engine := composition.NewEngine(composition.EngineConfig{
Store:          deps.Store,
ExecutorConfig: composition.DefaultExecutorConfig(),
})
return NewCompositionHandler(engine)
})
registerHandler("freshness", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewFreshnessHandler(freshness.NewManager(freshness.DefaultManagerConfig()))
})
registerHandler("migration", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewMigrationHandler(migration.NewManager(migration.DefaultManagerConfig()))
})
registerHandler("saas", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
reg := saas.NewPlanRegistry()
billing := saas.NewBillingManager(reg)
return NewSaaSHandler(reg, billing, saas.NewProvisioningManager(reg, billing))
})
registerHandler("gitops", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
loader := gitops.NewSchemaLoader(".")
policy := gitops.NewPolicyEngine()
return NewGitOpsHandler(loader, policy, gitops.NewSyncManager(loader, policy, nil, ".gitops-state.json"))
})
registerHandler("cost", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
tracker := cost.NewTracker("USD")
return NewCostHandler(tracker, cost.NewBudgetManager(tracker), cost.NewChargebackManager(tracker))
})
registerHandler("cluster", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
	// Cluster handler requires external cluster configuration.
	// Return nil to skip registration when deps are not available.
	return nil
})
registerHandler("scheduler", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewSchedulerHandler(warehouse.NewCronScheduler(nil, slog.Default()))
})
registerHandler("sla", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewSLAHandler(sla.NewManager(nil, sla.DefaultManagerConfig()))
})
registerHandler("drift", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewDriftHandler(drift.NewDetector(drift.DefaultConfig()))
})
registerHandler("lineage", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewLineageHandler(lineage.NewTracker())
})
registerHandler("semantic", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewSemanticHandler(semantic.NewSearch(semantic.NewLocalEmbedder(128), slog.Default()))
})
registerHandler("wasm", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewWASMHandler(wasm.NewRuntime(wasm.DefaultConfig(), slog.Default()))
})
registerHandler("federation", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewFederationHandler(federation.NewFederation(federation.DefaultConfig()))
})
registerHandler("quality", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewQualityHandler(quality.NewValidator())
})

// ╔══════════════════════════════════════════════════════════════════╗
// ║  EXPERIMENTAL — Working implementation, may be incomplete or    ║
// ║  change significantly. Use at your own risk.                    ║
// ╚══════════════════════════════════════════════════════════════════╝

registerHandler("graphql", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
if deps.Store == nil || deps.Schema == nil {
return nil
}
s, err := graphql.NewFeatureStoreSchema(deps.Store, deps.Schema)
if err != nil {
return nil
}
return NewGraphQLHandler(s)
})
registerHandler("autogen", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewAutogenHandler(autogen.NewGenerator(autogen.DefaultConfig()))
})
registerHandler("experiment", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewExperimentHandler(experiment.NewEngine())
})
registerHandler("ui", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
h, err := ui.NewHandler()
if err != nil {
return nil
}
return h
})
registerHandler("dbt", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewDBTHandler(deps.Config.Dependencies.DBTOptions)
})
registerHandler("compute", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewComputeHandler(compute.NewComputeEngine(compute.DefaultComputeConfig()))
})
registerHandler("consensus", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
node := consensus.NewRaftNode(consensus.DefaultRaftConfig(), nil)
return NewConsensusHandler(node, consensus.NewShardManager(16, node))
})
registerHandler("stream_sql", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewStreamSQLHandler(streamsql.NewEngine(streamsql.DefaultEngineConfig()))
})
registerHandler("control_plane", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewControlPlaneHandler(controlplane.NewManager(controlplane.DefaultManagerConfig()))
})
registerHandler("rag", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewRAGHandler(rag.NewPipeline(rag.DefaultPipelineConfig()))
})
registerHandler("plugin", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewPluginHandler(plugin.NewRegistry(plugin.DefaultRegistryConfig()))
})
registerHandler("versioning", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewVersioningHandler(versioning.NewVersionStore())
})
registerHandler("validation", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewValidationHandler(validation.NewValidator(validation.DefaultValidatorConfig()))
})
registerHandler("dashboard_v2", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewDashboardHandler(deps.Store, deps.Metrics)
})

registerHandler("sharding", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return nil
})

registerHandler("marketplace", MaturityStable, func(deps *handlerDeps) FeatureHandler {
return NewMarketplaceHandler()
})
registerHandler("billing", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewBillingHandler(marketplace.NewBillingEngine(marketplace.DefaultBillingConfig()))
})
registerHandler("cloud_service", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewCloudServiceHandler()
})
registerHandler("featherql", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewFeatherQLHandler()
})
registerHandler("llm_cache", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewLLMCacheHandler()
})
registerHandler("autofe", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewAutoFEHandler()
})
registerHandler("geo_routing", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewGeoRoutingHandler()
})
registerHandler("ab_rollout", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewABRolloutHandler()
})
registerHandler("edge_runtime", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewEdgeRuntimeHandler()
})
registerHandler("contracts", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewContractHandler(contract.NewManager(contract.DefaultManagerConfig(), nil))
})
registerHandler("materialization", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewMaterializationHandler(materialization.NewEngine(materialization.DefaultEngineConfig()))
})
registerHandler("playground", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewPlaygroundHandler(playground.NewService(nil))
})
registerHandler("replication", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewReplicationHandler(replication.NewManager(replication.DefaultManagerConfig()))
})
registerHandler("pushdown", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewPushdownHandler(pushdown.NewEvaluator())
})
registerHandler("llm_features", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewLLMFeatureHandler(llmfeature.NewStore(llmfeature.DefaultStoreConfig()))
})
registerHandler("finops", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewFinOpsHandler(finops.NewManager(finops.DefaultManagerConfig()))
})
registerHandler("parity", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewParityHandler(parity.NewChecker(parity.DefaultConfig()))
})
registerHandler("monitoring", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewMonitoringHandler(monitoring.NewManager(monitoring.DefaultManagerConfig()))
})
registerHandler("time_travel", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewTimeTravelHandler(timetravel.NewDebugger(timetravel.DefaultDebuggerConfig()))
})
registerHandler("catalog_ui", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewCatalogUIHandler(catalog.NewService(catalog.DefaultConfig()))
})
registerHandler("feather_cloud", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewCloudHandler(cloud.NewControlPlane(cloud.DefaultConfig()))
})
registerHandler("stream_dsl", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewStreamDSLHandler(streamdsl.NewPipelineManager(streamdsl.DefaultCompilerConfig()))
})
registerHandler("llm_gateway", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewLLMGatewayHandler(llmgateway.NewGateway(llmgateway.DefaultGatewayConfig()))
})
registerHandler("skew_detect", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewSkewDetectHandler(skewdetect.NewDetector(skewdetect.DefaultDetectorConfig()))
})
registerHandler("compute_graph", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewComputeGraphHandler(computegraph.NewEngine(computegraph.DefaultEngineConfig()))
})
registerHandler("k8s_autoscaler", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewAutoscalerHandler(autoscaler.NewAutoscaler(autoscaler.DefaultConfig()))
})
registerHandler("multi_region", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewMultiRegionHandler(multiregion.NewFederation(multiregion.DefaultFederationConfig()))
})
registerHandler("bench_suite", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewBenchSuiteHandler(benchsuite.NewSuite(benchsuite.DefaultSuiteConfig()))
})
registerHandler("ftl", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewFTLHandler(ftl.NewCompiler(ftl.DefaultCompilerConfig()))
})
registerHandler("quality_score", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewQualityScoreHandler(qualityscore.NewScorer(qualityscore.DefaultScoringConfig()))
})
registerHandler("explorer", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewExplorerHandler(dashboard.NewExplorer(dashboard.DefaultExplorerConfig()))
})
registerHandler("orchestrator", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewOrchestratorHandler(nil)
})
registerHandler("mobile_sync", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewMobileSyncHandler(mobilesync.NewSyncManager(mobilesync.DefaultSyncConfig()))
})
registerHandler("ml_integrations", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewMLIntegrationsHandler(
mlflow.NewTracker(mlflow.DefaultConfig()),
kubeflow.NewManager(kubeflow.DefaultConfig()),
airflow.NewProvider(airflow.DefaultConfig()),
)
})
registerHandler("smpc", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewSMPCHandler(federation.NewSMPCEngine(federation.DefaultSMPCConfig()))
})

// ╔══════════════════════════════════════════════════════════════════╗
// ║  NEXT-GEN — Advanced features for next-generation use cases.   ║
// ╚══════════════════════════════════════════════════════════════════╝

registerHandler("stream_compute", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewStreamComputeHandler(streamcompute.NewEngine(streamcompute.DefaultEngineConfig()))
})
registerHandler("sdk_codegen", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewSDKCodegenHandler(sdkcodegen.NewGenerator(sdkcodegen.DefaultGeneratorConfig()))
})
registerHandler("prompt_store", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewPromptStoreHandler(promptstore.NewStore(promptstore.DefaultStoreConfig()))
})
registerHandler("consistency_validator", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewConsistencyValidatorHandler(consistencyvalidator.NewValidator(consistencyvalidator.DefaultValidatorConfig()))
})
registerHandler("feature_dashboard", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewFeatureDashboardHandler(featuredashboard.NewDashboard(featuredashboard.DefaultDashboardConfig()))
})
registerHandler("featherql_v2", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewFeatherQLv2Handler(featherqlv2.NewEngine(featherqlv2.DefaultEngineConfig()))
})
registerHandler("embedding_mgmt", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewEmbeddingMgmtHandler(embeddingmgmt.NewManager(embeddingmgmt.DefaultManagerConfig()))
})
registerHandler("schema_evolution", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewSchemaEvolutionHandler(schemaevolution.NewManager(schemaevolution.DefaultManagerConfig()))
})
registerHandler("feast_compat", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewFeastCompatHandler(feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig()))
})
registerHandler("saas_control", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewSaaSControlHandler(saascontrol.NewControlPlane(saascontrol.DefaultControlPlaneConfig()))
})

// ╔══════════════════════════════════════════════════════════════════╗
// ║  NEXT-GEN v2 — Second wave of advanced features.               ║
// ╚══════════════════════════════════════════════════════════════════╝

registerHandler("lineage_graph", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewLineageGraphHandler(lineagegraph.NewGraph(lineagegraph.DefaultGraphConfig()))
})
registerHandler("adaptive_cache", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewAdaptiveCacheHandler(adaptivecache.NewPredictor(adaptivecache.DefaultPredictorConfig()))
})
registerHandler("contract_test", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewContractTestHandler(contracttest.NewRunner(contracttest.DefaultRunnerConfig()))
})
registerHandler("backpressure", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewBackpressureHandler(backpressure.NewMonitor(backpressure.DefaultMonitorConfig()))
})
registerHandler("offline_store", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewOfflineStoreHandler(offlinestore.NewStore(offlinestore.DefaultStoreConfig()))
})
registerHandler("bench_pub", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewBenchPubHandler(benchpub.NewSuite(benchpub.DefaultSuiteConfig()))
})
registerHandler("gitops_defs", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewGitOpsDefsHandler(gitopsdefs.NewReconciler(gitopsdefs.DefaultReconcilerConfig()))
})
registerHandler("ab_features", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewABFeaturesHandler(abfeatures.NewManager(abfeatures.DefaultExperimentConfig()))
})
registerHandler("federated_discovery", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewFederatedDiscoveryHandler(federateddiscovery.NewCatalog(federateddiscovery.DefaultCatalogConfig()))
})
registerHandler("wasm_udf", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewWasmUDFHandler(wasmudf.NewRuntime(wasmudf.DefaultRuntimeConfig()))
})

// ╔══════════════════════════════════════════════════════════════════╗
// ║  NEXT-GEN v3 — Third wave: operations, governance, platform.   ║
// ╚══════════════════════════════════════════════════════════════════╝

registerHandler("api_gateway", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewAPIGatewayHandler(apigateway.NewGateway(apigateway.DefaultGatewayConfig()))
})
registerHandler("importance_scoring", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewImportanceScoringHandler(importancescoring.NewScorer(importancescoring.DefaultScorerConfig()))
})
registerHandler("anomaly_detect", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewAnomalyDetectHandler(anomalydetect.NewDetector(anomalydetect.DefaultDetectorConfig()))
})
registerHandler("feather_cli", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewFeatherCLIHandler(feathercli.NewClient(feathercli.DefaultClientConfig()))
})
registerHandler("incr_materialization", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
return NewIncrMatHandler(incrmat.NewEngine(incrmat.DefaultEngineConfig()))
})
registerHandler("webhook_events", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewWebhooksHandler(webhooks.NewDispatcher(webhooks.DefaultDispatcherConfig()))
})
registerHandler("cloud_storage", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewCloudStorageHandler(cloudstorage.NewObjectStore(cloudstorage.DefaultStoreConfig()))
})
registerHandler("audit_log", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewAuditLogHandler(auditlog.NewLogger(auditlog.DefaultLoggerConfig()))
})
registerHandler("openapi_sync", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewOpenAPISyncHandler(openapisync.NewGenerator(openapisync.DefaultGeneratorConfig()))
})
registerHandler("terraform_provider", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
return NewTerraformProviderHandler(terraformprovider.NewProvider(terraformprovider.DefaultProviderConfig()))
})
}

// RegisteredFeatures returns all available feature names.
func RegisteredFeatures() []string {
names := make([]string, 0, len(featureRegistry))
for name := range featureRegistry {
names = append(names, name)
}
return names
}

// RegisteredHandlerSpecs returns handler specs grouped by maturity.
// Useful for CLI tools and diagnostics (e.g., make api-routes).
func RegisteredHandlerSpecs() []HandlerSpec {
out := make([]HandlerSpec, len(handlerSpecs))
copy(out, handlerSpecs)
return out
}

// registerEnabledFeatures creates and registers all enabled feature handlers.
func registerEnabledFeatures(mux *http.ServeMux, enabled map[string]bool, deps *handlerDeps) {
// Validate that all enabled feature names correspond to registered handlers
for name := range enabled {
if !enabled[name] {
continue
}
if _, exists := featureRegistry[name]; !exists {
slog.Warn("enabled feature has no registered handler, ignoring", "feature", name)
}
}

for name, factory := range featureRegistry {
if !enabled[name] {
continue
}
handler := factory(deps)
if handler == nil {
slog.Warn("feature handler factory returned nil, skipping", "handler", name)
continue
}
handler.RegisterRoutes(mux)
}
}

// HandlerInventory describes a registered handler for API documentation.
type HandlerInventory struct {
	Name     string `json:"name"`
	Maturity string `json:"maturity"`
	Enabled  bool   `json:"enabled"`
}

// GetHandlerInventory returns all handlers with maturity and enabled status.
func GetHandlerInventory(enabled map[string]bool) []HandlerInventory {
	inv := make([]HandlerInventory, 0, len(handlerSpecs))
	for _, spec := range handlerSpecs {
		inv = append(inv, HandlerInventory{
			Name:     spec.Name,
			Maturity: string(spec.Maturity),
			Enabled:  enabled[spec.Name],
		})
	}
	return inv
}

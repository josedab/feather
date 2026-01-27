package server

import (
	"github.com/feather-store/feather/internal/extensions/abfeatures"
	"github.com/feather-store/feather/internal/extensions/adaptivecache"
	"github.com/feather-store/feather/internal/extensions/anomalydetect"
	"github.com/feather-store/feather/internal/extensions/apigateway"
	"github.com/feather-store/feather/internal/extensions/auditlog"
	"github.com/feather-store/feather/internal/extensions/backpressure"
	"github.com/feather-store/feather/internal/extensions/benchpub"
	"github.com/feather-store/feather/internal/extensions/cloudstorage"
	"github.com/feather-store/feather/internal/extensions/consistencyvalidator"
	"github.com/feather-store/feather/internal/extensions/contracttest"
	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
	"github.com/feather-store/feather/internal/extensions/feastcompat"
	"github.com/feather-store/feather/internal/extensions/featuredashboard"
	"github.com/feather-store/feather/internal/extensions/feathercli"
	"github.com/feather-store/feather/internal/extensions/featherqlv2"
	"github.com/feather-store/feather/internal/extensions/federateddiscovery"
	"github.com/feather-store/feather/internal/extensions/ftl"
	"github.com/feather-store/feather/internal/extensions/gitopsdefs"
	"github.com/feather-store/feather/internal/extensions/importancescoring"
	"github.com/feather-store/feather/internal/extensions/incrmat"
	"github.com/feather-store/feather/internal/extensions/lineagegraph"
	"github.com/feather-store/feather/internal/extensions/mobilesync"
	"github.com/feather-store/feather/internal/extensions/offlinestore"
	"github.com/feather-store/feather/internal/extensions/openapisync"
	"github.com/feather-store/feather/internal/extensions/promptstore"
	"github.com/feather-store/feather/internal/extensions/pythonsdk"
	"github.com/feather-store/feather/internal/extensions/qualityscore"
	"github.com/feather-store/feather/internal/extensions/saascontrol"
	"github.com/feather-store/feather/internal/extensions/schemaevolution"
	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
	"github.com/feather-store/feather/internal/extensions/streamcompute"
	"github.com/feather-store/feather/internal/extensions/terraformprovider"
	"github.com/feather-store/feather/internal/extensions/wasmudf"
	"github.com/feather-store/feather/internal/extensions/webhooks"
	"github.com/feather-store/feather/internal/integrations/airflow"
	"github.com/feather-store/feather/internal/integrations/flinkpipeline"
	"github.com/feather-store/feather/internal/integrations/kubeflow"
	"github.com/feather-store/feather/internal/integrations/mlflow"
	"github.com/feather-store/feather/internal/platform/autoscaler"
	"github.com/feather-store/feather/internal/platform/cloudcontrol"
	"github.com/feather-store/feather/internal/platform/finops"
	"github.com/feather-store/feather/internal/platform/realtimemonitor"
	"github.com/feather-store/feather/internal/platform/monitoring"
	"github.com/feather-store/feather/internal/platform/multiregion"
	"github.com/feather-store/feather/internal/platform/parity"
	"github.com/feather-store/feather/internal/platform/federation"
	"github.com/feather-store/feather/internal/tools/benchsuite"
	"github.com/feather-store/feather/internal/tools/dashboard"
)

// Handler registrations: Next-gen features — Advanced features for next-generation use cases.

func init() {
	// Next-gen v1
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
	registerHandler("feast_gateway", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		adapter := feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig())
		return NewFeastGatewayHandler(feastcompat.NewGateway(adapter))
	})
	registerHandler("saas_control", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewSaaSControlHandler(saascontrol.NewControlPlane(saascontrol.DefaultControlPlaneConfig()))
	})

	// Next-gen v2
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

	// Next-gen v3: operations, governance, platform
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

	// Platform & tools
	registerHandler("finops", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewFinOpsHandler(finops.NewManager(finops.DefaultManagerConfig()))
	})
	registerHandler("parity", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewParityHandler(parity.NewChecker(parity.DefaultConfig()))
	})
	registerHandler("monitoring", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewMonitoringHandler(monitoring.NewManager(monitoring.DefaultManagerConfig()))
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

	// Streaming feature pipelines (Flink/Kafka Streams)
	registerHandler("flink_pipeline", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewFlinkPipelineHandler(flinkpipeline.NewManager(flinkpipeline.DefaultManagerConfig()))
	})

	// Managed cloud control plane
	registerHandler("cloud_control", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewCloudControlHandler(cloudcontrol.NewControlPlane(cloudcontrol.DefaultControlPlaneConfig()))
	})

	// Real-time monitoring dashboard
	registerHandler("realtime_monitor", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewRealtimeMonitorHandler(realtimemonitor.NewDashboard(realtimemonitor.DefaultDashboardConfig()))
	})

	// CDC incremental materialization
	registerHandler("cdc_materialization", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewCDCHandler(incrmat.NewEngine(incrmat.DefaultEngineConfig()))
	})

	// Python feature transform SDK
	registerHandler("python_transforms", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewPythonSDKHandler(pythonsdk.NewRegistry(pythonsdk.DefaultRegistryConfig()))
	})
}

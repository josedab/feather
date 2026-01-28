package server

import (
	"github.com/feather-store/feather/internal/extensions/ftl"
	"github.com/feather-store/feather/internal/extensions/incrmat"
	"github.com/feather-store/feather/internal/extensions/lineageanalysis"
	"github.com/feather-store/feather/internal/extensions/llmstore"
	"github.com/feather-store/feather/internal/extensions/mobilesync"
	"github.com/feather-store/feather/internal/extensions/pythonsdk"
	"github.com/feather-store/feather/internal/extensions/qualityscore"
	"github.com/feather-store/feather/internal/extensions/wasmruntime"
	"github.com/feather-store/feather/internal/integrations/airflow"
	"github.com/feather-store/feather/internal/integrations/flinkpipeline"
	"github.com/feather-store/feather/internal/integrations/kubeflow"
	"github.com/feather-store/feather/internal/integrations/mlflow"
	"github.com/feather-store/feather/internal/platform/autoscaler"
	"github.com/feather-store/feather/internal/platform/cloudcontrol"
	"github.com/feather-store/feather/internal/platform/federation"
	"github.com/feather-store/feather/internal/platform/finops"
	"github.com/feather-store/feather/internal/platform/monitoring"
	"github.com/feather-store/feather/internal/platform/multiregion"
	"github.com/feather-store/feather/internal/platform/parity"
	"github.com/feather-store/feather/internal/platform/realtimemonitor"
	"github.com/feather-store/feather/internal/tools/benchsuite"
	"github.com/feather-store/feather/internal/tools/dashboard"
)

// Handler registrations: Next-gen platform — Platform, tools, and integration features.

func init() {
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

	// Multi-region federation
	registerHandler("region_federation", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewRegionFederationHandler(multiregion.NewFederation(multiregion.DefaultFederationConfig()))
	})

	// Feature lineage & impact analysis
	registerHandler("lineage_analysis", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewLineageAnalysisHandler(lineageanalysis.NewTracker(lineageanalysis.DefaultTrackerConfig()))
	})

	// Serverless edge runtime (WASM)
	registerHandler("wasm_runtime", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewWasmRuntimeHandler(wasmruntime.NewEdgeManager(wasmruntime.DefaultEdgeManagerConfig()))
	})

	// LLM Feature Store (Prompt/Embedding/RAG)
	registerHandler("llm_store", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewLLMStoreHandler(llmstore.NewStore(llmstore.DefaultStoreConfig()))
	})
}

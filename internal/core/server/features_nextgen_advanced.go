package server

import (
	"github.com/feather-store/feather/internal/extensions/arrowflight"
	"github.com/feather-store/feather/internal/extensions/audittrail"
	"github.com/feather-store/feather/internal/extensions/backfillengine"
	"github.com/feather-store/feather/internal/extensions/compression"
	"github.com/feather-store/feather/internal/extensions/computegraph"
	"github.com/feather-store/feather/internal/extensions/contractcicd"
	"github.com/feather-store/feather/internal/extensions/diffprivacy"
	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
	"github.com/feather-store/feather/internal/extensions/feastcompat"
	"github.com/feather-store/feather/internal/extensions/fedlearning"
	"github.com/feather-store/feather/internal/extensions/garbagecollect"
	"github.com/feather-store/feather/internal/extensions/gitopsdefs"
	"github.com/feather-store/feather/internal/extensions/graphqlfederation"
	"github.com/feather-store/feather/internal/extensions/incrmat"
	"github.com/feather-store/feather/internal/extensions/lifecycle"
	"github.com/feather-store/feather/internal/extensions/modelserving"
	"github.com/feather-store/feather/internal/extensions/notebooksdk"
	"github.com/feather-store/feather/internal/extensions/obsconsole"
	"github.com/feather-store/feather/internal/extensions/offlinestore"
	"github.com/feather-store/feather/internal/extensions/playgroundv2"
	"github.com/feather-store/feather/internal/extensions/prefetch"
	"github.com/feather-store/feather/internal/extensions/pythonsdk"
	"github.com/feather-store/feather/internal/extensions/qualitygates"
	"github.com/feather-store/feather/internal/extensions/queryplanner"
	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
	"github.com/feather-store/feather/internal/extensions/semantic"
	"github.com/feather-store/feather/internal/extensions/starlarkudf"
	"github.com/feather-store/feather/internal/extensions/streamcompute"
	"github.com/feather-store/feather/internal/extensions/streamingcdc"
	"github.com/feather-store/feather/internal/platform/cluster"
	"github.com/feather-store/feather/internal/platform/federation"
	"github.com/feather-store/feather/internal/platform/tenant"
	"github.com/feather-store/feather/internal/platform/transform"
	"github.com/feather-store/feather/internal/tools/mcp"
)

// Handler registrations: Advanced next-gen features — consolidated from v4–v8.
//
// Categories: data transport, streaming & CDC, computation graphs, data processing,
// performance optimization, model & ML, privacy & governance, runtime & transforms,
// developer tools, federation, lifecycle & catalog, compatibility, infrastructure.
func init() {

	// ── Data transport & Arrow Flight ──────────────────────────────────

	registerHandler("arrow_flight", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewArrowFlightHandler(arrowflight.NewServer(arrowflight.DefaultConfig()))
	})
	registerHandler("arrow_batch", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		server := arrowflight.NewServer(arrowflight.DefaultConfig())
		bs := arrowflight.NewBatchServer(server, arrowflight.DefaultBatchConfig())
		conv := arrowflight.NewBatchConverter()
		return NewArrowBatchHandler(bs, conv)
	})
	registerHandler("flight_endpoint", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		server := arrowflight.NewServer(arrowflight.DefaultConfig())
		bs := arrowflight.NewBatchServer(server, arrowflight.DefaultBatchConfig())
		endpoint := arrowflight.NewFlightServiceEndpoint(server, bs, arrowflight.DefaultFlightServiceConfig())
		return NewFlightEndpointHandler(endpoint)
	})
	registerHandler("arrow_flight_batch", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		server := arrowflight.NewServer(arrowflight.DefaultConfig())
		bs := arrowflight.NewBatchServer(server, arrowflight.DefaultBatchConfig())
		return NewArrowFlightBatchHandler(server, bs)
	})

	// ── Streaming & CDC ────────────────────────────────────────────────

	registerHandler("stream_advanced", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		proc := streamcompute.NewExactlyOnceProcessor(streamcompute.DefaultExactlyOnceConfig())
		return NewStreamAdvancedHandler(proc)
	})
	registerHandler("streaming_cdc", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewStreamingCDCHandler(streamingcdc.NewManager(streamingcdc.DefaultManagerConfig()))
	})
	registerHandler("streaming_cdc_pipeline", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		engine := incrmat.NewEngine(incrmat.DefaultEngineConfig())
		cdcMgr := incrmat.NewCDCManager(engine, 100000)
		recovery := incrmat.NewRecoveryManager(cdcMgr, incrmat.DefaultRecoveryConfig())
		return NewStreamingCDCPipelineHandler(cdcMgr, engine, recovery)
	})

	// ── Computation graphs ─────────────────────────────────────────────

	registerHandler("compute_graph_v2", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
		memoizer := computegraph.NewMemoizer(computegraph.DefaultMemoizerConfig())
		return NewComputeGraphV2Handler(engine, memoizer)
	})
	registerHandler("declarative_graph", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
		memoizer := computegraph.NewMemoizer(computegraph.DefaultMemoizerConfig())
		graph := computegraph.NewDeclarativeGraph(engine, memoizer)
		return NewDeclarativeGraphHandler(graph)
	})
	registerHandler("computation_graph", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
		memoizer := computegraph.NewMemoizer(computegraph.DefaultMemoizerConfig())
		return NewComputationGraphHandler(engine, memoizer)
	})

	// ── Data processing & storage ──────────────────────────────────────

	registerHandler("compression", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewCompressionHandler(compression.NewSelector(compression.DefaultConfig()))
	})
	registerHandler("backfill_engine", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		coord := backfillengine.NewCoordinator(backfillengine.DefaultCoordinatorConfig())
		return NewBackfillEngineHandler(coord)
	})
	registerHandler("consistency_advanced", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewConsistencyAdvancedHandler()
	})
	registerHandler("offline_store_sync", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		store := offlinestore.NewStore(offlinestore.DefaultStoreConfig())
		syncer := offlinestore.NewWarehouseSyncer(offlinestore.DefaultWarehouseSyncConfig(), store)
		return NewOfflineStoreSyncHandler(store, syncer)
	})

	// ── Performance optimization ───────────────────────────────────────

	registerHandler("prefetch", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPrefetchHandler(prefetch.NewController(prefetch.DefaultConfig()))
	})
	registerHandler("predictive_warming", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		forecaster := prefetch.NewForecaster(prefetch.DefaultForecasterConfig())
		warmer := prefetch.NewWarmer(prefetch.DefaultWarmerConfig(), forecaster)
		return NewPredictiveWarmingHandler(forecaster, warmer)
	})
	registerHandler("predictive_prefetch", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		controller := prefetch.NewController(prefetch.DefaultConfig())
		forecaster := prefetch.NewForecaster(prefetch.DefaultForecasterConfig())
		return NewPredictivePrefetchHandler(controller, forecaster)
	})

	// ── Model & ML ─────────────────────────────────────────────────────

	registerHandler("fed_learning", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewFedLearningHandler(fedlearning.NewAdapter(fedlearning.DefaultConfig()))
	})
	registerHandler("model_gateway", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		reg := modelserving.NewRegistry(modelserving.DefaultRegistryConfig())
		gw := modelserving.NewGateway(reg)
		return NewModelGatewayHandler(gw)
	})
	registerHandler("embedding_lifecycle", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		mgr := embeddingmgmt.NewManager(embeddingmgmt.DefaultManagerConfig())
		bp := embeddingmgmt.NewBatchProcessor(mgr, embeddingmgmt.DefaultBatchConfig())
		ab := embeddingmgmt.NewABTester(mgr)
		drift := embeddingmgmt.NewVectorDriftDetector(embeddingmgmt.DefaultVectorDriftConfig())
		return NewEmbeddingLifecycleHandler(bp, ab, drift)
	})

	// ── Privacy, governance & contracts ─────────────────────────────────

	registerHandler("diff_privacy", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewDiffPrivacyHandler(diffprivacy.NewEngine(diffprivacy.DefaultConfig()))
	})
	registerHandler("quality_gates", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewQualityGatesHandler(qualitygates.NewValidator(qualitygates.DefaultConfig()))
	})
	registerHandler("audit_trail", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewAuditTrailHandler(audittrail.New(audittrail.DefaultConfig()))
	})
	registerHandler("contract_cicd", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewContractCICDHandler(contractcicd.NewEngine(contractcicd.DefaultEngineConfig()))
	})
	registerHandler("contract_cicd_native", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewContractCICDNativeHandler(contractcicd.NewEngine(contractcicd.DefaultEngineConfig()))
	})

	// ── Runtime & transforms ───────────────────────────────────────────

	registerHandler("starlark_udf", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewStarlarkUDFHandler(starlarkudf.NewRegistry(starlarkudf.DefaultRegistryConfig()))
	})
	registerHandler("python_runtime", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPythonRuntimeHandler(transform.NewPythonExecutor())
	})
	registerHandler("python_sidecar", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		registry := pythonsdk.NewRegistry(pythonsdk.DefaultRegistryConfig())
		mgr := pythonsdk.NewSidecarManager(pythonsdk.DefaultSidecarConfig(), registry)
		return NewPythonSidecarHandler(mgr)
	})
	registerHandler("query_planner", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewQueryPlannerHandler(queryplanner.New(queryplanner.DefaultConfig()))
	})

	// ── Developer tools ────────────────────────────────────────────────

	registerHandler("notebook_sdk", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewNotebookSDKHandler(notebooksdk.NewService(notebooksdk.DefaultConfig()))
	})
	registerHandler("playground_v2", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPlaygroundV2Handler(playgroundv2.NewEnvironment(playgroundv2.DefaultConfig(), nil))
	})
	registerHandler("sdk_languages", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewSDKLanguagesHandler(sdkcodegen.NewLanguageRegistry())
	})
	registerHandler("notebook_sdk_v2", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewNotebookSDKv2Handler(notebooksdk.NewService(notebooksdk.DefaultConfig()))
	})
	registerHandler("mcp_server", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewMCPServerHandler(mcp.GetServerInfo(), mcp.BuiltinResources(), mcp.BuiltinPrompts())
	})

	// ── Federation & multi-org ─────────────────────────────────────────

	registerHandler("federation_cross_org", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewCrossOrgFederationHandler(federation.NewCrossOrgFederation(federation.DefaultCrossOrgConfig()))
	})
	registerHandler("cross_org_federation", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewCrossOrgFedHandler(federation.NewCrossOrgFederation(federation.DefaultCrossOrgConfig()))
	})
	registerHandler("graphql_federation", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewGraphQLFederationHandler(graphqlfederation.NewGateway(graphqlfederation.DefaultGatewayConfig()))
	})

	// ── Lifecycle, catalog & maintenance ────────────────────────────────

	registerHandler("semantic_catalog", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewSemanticCatalogHandler(semantic.NewCatalog(semantic.DefaultCatalogConfig()))
	})
	registerHandler("lifecycle_manager", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewLifecycleManagerHandler(lifecycle.NewManager(lifecycle.DefaultManagerConfig()))
	})
	registerHandler("feature_gc", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewGarbageCollectHandler(garbagecollect.NewCollector())
	})

	// ── Compatibility & integration ────────────────────────────────────

	registerHandler("feast_enhanced", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		svcMgr := feastcompat.NewFeatureServiceManager(feastcompat.DefaultFeatureServiceConfig())
		adapter := feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig())
		gw := feastcompat.NewGateway(adapter)
		suite := feastcompat.NewCompatTestSuite(gw)
		migration := feastcompat.NewMigrationCLI()
		return NewFeastEnhancedHandler(svcMgr, suite, migration)
	})
	registerHandler("feast_ga", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		gaGw := feastcompat.NewGAGateway(feastcompat.DefaultGAConfig())
		if deps.Store != nil {
			storeAdapter := feastcompat.NewStoreLookupAdapter(deps.Store)
			gaGw.SetStoreAdapter(storeAdapter)
		}
		return NewFeastGAHandler(gaGw)
	})
	registerHandler("gitops_manifests", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewGitOpsManifestHandler(gitopsdefs.NewManifestLoader())
	})

	// ── Infrastructure & platform ──────────────────────────────────────

	registerHandler("multi_tenant_metering", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		meter := tenant.NewUsageMeter()
		return NewMultiTenantHandler(meter, tenant.DefaultCostConfig())
	})
	registerHandler("auto_sharding", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewAutoShardingHandler(cluster.NewAutoShardingEngine(cluster.DefaultAutoShardingConfig()))
	})
	registerHandler("obs_console", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewObsConsoleHandler(obsconsole.NewConsole(obsconsole.DefaultConsoleConfig()))
	})
}

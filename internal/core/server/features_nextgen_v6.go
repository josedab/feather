package server

import (
	"github.com/feather-store/feather/internal/extensions/arrowflight"
	"github.com/feather-store/feather/internal/extensions/computegraph"
	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
	"github.com/feather-store/feather/internal/extensions/feastcompat"
	"github.com/feather-store/feather/internal/extensions/gitopsdefs"
	"github.com/feather-store/feather/internal/extensions/prefetch"
	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
	"github.com/feather-store/feather/internal/extensions/streamcompute"
	"github.com/feather-store/feather/internal/platform/transform"
)

// Handler registrations: Next-gen v6 — Python runtime, compute graph v2,
// advanced consistency, GitOps manifests, Arrow batch, stream advanced,
// Feast enhanced, embedding lifecycle, SDK languages, predictive warming.
//
// Registers: python_runtime, compute_graph_v2, consistency_advanced,
// gitops_manifests, arrow_batch, stream_advanced, feast_enhanced,
// embedding_lifecycle, sdk_languages, predictive_warming.
func init() {
	registerHandler("python_runtime", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPythonRuntimeHandler(transform.NewPythonExecutor())
	})
	registerHandler("compute_graph_v2", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
		memoizer := computegraph.NewMemoizer(computegraph.DefaultMemoizerConfig())
		return NewComputeGraphV2Handler(engine, memoizer)
	})
	registerHandler("consistency_advanced", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewConsistencyAdvancedHandler()
	})
	registerHandler("gitops_manifests", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewGitOpsManifestHandler(gitopsdefs.NewManifestLoader())
	})
	registerHandler("arrow_batch", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		server := arrowflight.NewServer(arrowflight.DefaultConfig())
		bs := arrowflight.NewBatchServer(server, arrowflight.DefaultBatchConfig())
		conv := arrowflight.NewBatchConverter()
		return NewArrowBatchHandler(bs, conv)
	})
	registerHandler("stream_advanced", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		proc := streamcompute.NewExactlyOnceProcessor(streamcompute.DefaultExactlyOnceConfig())
		return NewStreamAdvancedHandler(proc)
	})
	registerHandler("feast_enhanced", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		svcMgr := feastcompat.NewFeatureServiceManager(feastcompat.DefaultFeatureServiceConfig())
		adapter := feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig())
		gw := feastcompat.NewGateway(adapter)
		suite := feastcompat.NewCompatTestSuite(gw)
		migration := feastcompat.NewMigrationCLI()
		return NewFeastEnhancedHandler(svcMgr, suite, migration)
	})
	registerHandler("embedding_lifecycle", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		mgr := embeddingmgmt.NewManager(embeddingmgmt.DefaultManagerConfig())
		bp := embeddingmgmt.NewBatchProcessor(mgr, embeddingmgmt.DefaultBatchConfig())
		ab := embeddingmgmt.NewABTester(mgr)
		drift := embeddingmgmt.NewVectorDriftDetector(embeddingmgmt.DefaultVectorDriftConfig())
		return NewEmbeddingLifecycleHandler(bp, ab, drift)
	})
	registerHandler("sdk_languages", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewSDKLanguagesHandler(sdkcodegen.NewLanguageRegistry())
	})
	registerHandler("predictive_warming", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		forecaster := prefetch.NewForecaster(prefetch.DefaultForecasterConfig())
		warmer := prefetch.NewWarmer(prefetch.DefaultWarmerConfig(), forecaster)
		return NewPredictiveWarmingHandler(forecaster, warmer)
	})
}

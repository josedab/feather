package server

import (
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
)

// Handler registrations: Next-gen v2 — Data quality, lineage, and discovery features.

func init() {
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
}

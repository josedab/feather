package server

import (
	"github.com/feather-store/feather/internal/extensions/autogen"
	"github.com/feather-store/feather/internal/extensions/computegraph"
	"github.com/feather-store/feather/internal/extensions/graphql"
	"github.com/feather-store/feather/internal/extensions/llmfeature"
	"github.com/feather-store/feather/internal/extensions/llmgateway"
	"github.com/feather-store/feather/internal/extensions/rag"
	"github.com/feather-store/feather/internal/extensions/skewdetect"
	"github.com/feather-store/feather/internal/extensions/streamdsl"
	"github.com/feather-store/feather/internal/extensions/timetravel"
	"github.com/feather-store/feather/internal/platform/cloud"
	"github.com/feather-store/feather/internal/platform/gitops"
	"github.com/feather-store/feather/internal/platform/plugin"
	"github.com/feather-store/feather/internal/platform/pushdown"
	"github.com/feather-store/feather/internal/tools/catalog"
	"github.com/feather-store/feather/internal/tools/playground"
	"github.com/feather-store/feather/internal/tools/ui"
)

// Handler registrations: Experimental — Working implementation, may be incomplete or
// change significantly. Use at your own risk.
//
// Registers: graphql, autogen, ui, rag, plugin, playground, pushdown, llm_features,
// gitops, time_travel, catalog_ui, feather_cloud, stream_dsl, llm_gateway,
// skew_detect, compute_graph.
func init() {
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
	registerHandler("ui", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		h, err := ui.NewHandler()
		if err != nil {
			return nil
		}
		return h
	})
	registerHandler("rag", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewRAGHandler(rag.NewPipeline(rag.DefaultPipelineConfig()))
	})
	registerHandler("plugin", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPluginHandler(plugin.NewRegistry(plugin.DefaultRegistryConfig()))
	})
	registerHandler("playground", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPlaygroundHandler(playground.NewService(nil))
	})
	registerHandler("pushdown", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPushdownHandler(pushdown.NewEvaluator())
	})
	registerHandler("llm_features", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewLLMFeatureHandler(llmfeature.NewStore(llmfeature.DefaultStoreConfig()))
	})
	registerHandler("gitops", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		loader := gitops.NewSchemaLoader(".")
		policy := gitops.NewPolicyEngine()
		return NewGitOpsHandler(loader, policy, gitops.NewSyncManager(loader, policy, nil, ".gitops-state.json"))
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
}

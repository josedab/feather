package server

import (
	"github.com/feather-store/feather/internal/extensions/arrowflight"
	"github.com/feather-store/feather/internal/extensions/computegraph"
	"github.com/feather-store/feather/internal/extensions/contractcicd"
	"github.com/feather-store/feather/internal/extensions/garbagecollect"
	"github.com/feather-store/feather/internal/extensions/graphqlfederation"
	"github.com/feather-store/feather/internal/extensions/incrmat"
	"github.com/feather-store/feather/internal/extensions/notebooksdk"
	"github.com/feather-store/feather/internal/extensions/prefetch"
	"github.com/feather-store/feather/internal/extensions/streamingcdc"
	"github.com/feather-store/feather/internal/platform/federation"
	"github.com/feather-store/feather/internal/tools/mcp"
)

// Handler registrations: Next-gen v8 — Streaming CDC materialization,
// MCP server, zero-copy Arrow Flight batch, declarative computation graph,
// contract CI/CD, cross-org federation, predictive prefetching,
// notebook SDK, smart GC, GraphQL federation gateway.
func init() {
	registerHandler("streaming_cdc", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewStreamingCDCHandler(streamingcdc.NewManager(streamingcdc.DefaultManagerConfig()))
	})
	registerHandler("mcp_server", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewMCPServerHandler(mcp.GetServerInfo(), mcp.BuiltinResources(), mcp.BuiltinPrompts())
	})
	registerHandler("arrow_flight_batch", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		server := arrowflight.NewServer(arrowflight.DefaultConfig())
		bs := arrowflight.NewBatchServer(server, arrowflight.DefaultBatchConfig())
		return NewArrowFlightBatchHandler(server, bs)
	})
	registerHandler("computation_graph", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
		memoizer := computegraph.NewMemoizer(computegraph.DefaultMemoizerConfig())
		return NewComputationGraphHandler(engine, memoizer)
	})
	registerHandler("contract_cicd_native", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewContractCICDNativeHandler(contractcicd.NewEngine(contractcicd.DefaultEngineConfig()))
	})
	registerHandler("cross_org_federation", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewCrossOrgFedHandler(federation.NewCrossOrgFederation(federation.DefaultCrossOrgConfig()))
	})
	registerHandler("predictive_prefetch", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		controller := prefetch.NewController(prefetch.DefaultConfig())
		forecaster := prefetch.NewForecaster(prefetch.DefaultForecasterConfig())
		return NewPredictivePrefetchHandler(controller, forecaster)
	})
	registerHandler("notebook_sdk_v2", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewNotebookSDKv2Handler(notebooksdk.NewService(notebooksdk.DefaultConfig()))
	})
	registerHandler("feature_gc", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewGarbageCollectHandler(garbagecollect.NewCollector())
	})
	registerHandler("graphql_federation", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewGraphQLFederationHandler(graphqlfederation.NewGateway(graphqlfederation.DefaultGatewayConfig()))
	})

	registerHandler("streaming_cdc_pipeline", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		engine := incrmat.NewEngine(incrmat.DefaultEngineConfig())
		cdcMgr := incrmat.NewCDCManager(engine, 100000)
		recovery := incrmat.NewRecoveryManager(cdcMgr, incrmat.DefaultRecoveryConfig())
		return NewStreamingCDCPipelineHandler(cdcMgr, engine, recovery)
	})
}

package server

import (
	"github.com/feather-store/feather/internal/extensions/arrowflight"
	"github.com/feather-store/feather/internal/extensions/backfillengine"
	"github.com/feather-store/feather/internal/extensions/computegraph"
	"github.com/feather-store/feather/internal/extensions/contractcicd"
	"github.com/feather-store/feather/internal/extensions/feastcompat"
	"github.com/feather-store/feather/internal/extensions/obsconsole"
	"github.com/feather-store/feather/internal/extensions/offlinestore"
	"github.com/feather-store/feather/internal/extensions/pythonsdk"
	"github.com/feather-store/feather/internal/platform/cluster"
	"github.com/feather-store/feather/internal/platform/tenant"
)

// Handler registrations: Next-gen v7 — Unified streaming backfill engine,
// feature contract CI/CD, declarative computation graph, multi-tenant
// isolation with metering, Python transform runtime, unified observability
// console, horizontal auto-sharding, Feast-compatible gateway GA, offline
// store with warehouse sync, Arrow Flight batch serving.
func init() {
	registerHandler("backfill_engine", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		coord := backfillengine.NewCoordinator(backfillengine.DefaultCoordinatorConfig())
		return NewBackfillEngineHandler(coord)
	})
	registerHandler("contract_cicd", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewContractCICDHandler(contractcicd.NewEngine(contractcicd.DefaultEngineConfig()))
	})
	registerHandler("flight_endpoint", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		server := arrowflight.NewServer(arrowflight.DefaultConfig())
		bs := arrowflight.NewBatchServer(server, arrowflight.DefaultBatchConfig())
		endpoint := arrowflight.NewFlightServiceEndpoint(server, bs, arrowflight.DefaultFlightServiceConfig())
		return NewFlightEndpointHandler(endpoint)
	})
	registerHandler("declarative_graph", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		engine := computegraph.NewEngine(computegraph.DefaultEngineConfig())
		memoizer := computegraph.NewMemoizer(computegraph.DefaultMemoizerConfig())
		graph := computegraph.NewDeclarativeGraph(engine, memoizer)
		return NewDeclarativeGraphHandler(graph)
	})
	registerHandler("multi_tenant_metering", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		meter := tenant.NewUsageMeter()
		return NewMultiTenantHandler(meter, tenant.DefaultCostConfig())
	})
	registerHandler("python_sidecar", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		registry := pythonsdk.NewRegistry(pythonsdk.DefaultRegistryConfig())
		mgr := pythonsdk.NewSidecarManager(pythonsdk.DefaultSidecarConfig(), registry)
		return NewPythonSidecarHandler(mgr)
	})
	registerHandler("obs_console", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewObsConsoleHandler(obsconsole.NewConsole(obsconsole.DefaultConsoleConfig()))
	})
	registerHandler("auto_sharding", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewAutoShardingHandler(cluster.NewAutoShardingEngine(cluster.DefaultAutoShardingConfig()))
	})
	registerHandler("feast_ga", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewFeastGAHandler(feastcompat.NewGAGateway(feastcompat.DefaultGAConfig()))
	})
	registerHandler("offline_store_sync", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		store := offlinestore.NewStore(offlinestore.DefaultStoreConfig())
		syncer := offlinestore.NewWarehouseSyncer(offlinestore.DefaultWarehouseSyncConfig(), store)
		return NewOfflineStoreSyncHandler(store, syncer)
	})
}

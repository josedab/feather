package server

import (
	"context"
	"log/slog"

	"github.com/feather-store/feather/internal/extensions/composition"
	"github.com/feather-store/feather/internal/extensions/experiment"
	"github.com/feather-store/feather/internal/extensions/lineage"
	"github.com/feather-store/feather/internal/extensions/marketplace"
	"github.com/feather-store/feather/internal/extensions/materialization"
	"github.com/feather-store/feather/internal/extensions/versioning"
	"github.com/feather-store/feather/internal/extensions/wasm"
	"github.com/feather-store/feather/internal/integrations/streamsql"
	"github.com/feather-store/feather/internal/integrations/warehouse"
	"github.com/feather-store/feather/internal/platform/consensus"
	"github.com/feather-store/feather/internal/platform/contract"
	"github.com/feather-store/feather/internal/platform/controlplane"
	"github.com/feather-store/feather/internal/platform/cost"
	"github.com/feather-store/feather/internal/platform/federation"
	"github.com/feather-store/feather/internal/platform/migration"
	"github.com/feather-store/feather/internal/platform/replication"
	"github.com/feather-store/feather/internal/platform/saas"
	"github.com/feather-store/feather/internal/platform/validation"
	"github.com/feather-store/feather/internal/tools/compute"
)

// Handler registrations: Beta — Functional and tested, API may change between minor
// releases. Suitable for staging and non-critical production.
//
// Registers: tenant, warehouse, embedding, composition, migration, saas, cost,
// cluster, scheduler, lineage, wasm, federation, experiment, dbt, compute,
// consensus, stream_sql, control_plane, versioning, validation, dashboard_v2,
// billing, cloud_service, featherql, llm_cache, autofe, geo_routing, ab_rollout,
// edge_runtime, contracts, materialization, replication.
func init() {
	registerHandler("tenant", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		maxBytes := deps.Config.Core.TenantMaxBytes
		if maxBytes == 0 {
			maxBytes = 4 * 1024 * 1024 * 1024
		}
		h := NewTenantHandler(maxBytes)
		h.requireAuth = deps.AuthMiddleware
		return h
	})
	registerHandler("warehouse", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewWarehouseHandler(WarehouseHandlerConfig{
			Store:  deps.Store,
			Schema: deps.Schema,
		})
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
	registerHandler("migration", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		h := NewMigrationHandler(migration.NewManager(migration.DefaultManagerConfig()))
		h.requireAuth = deps.AuthMiddleware
		return h
	})
	registerHandler("saas", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		reg := saas.NewPlanRegistry()
		billing := saas.NewBillingManager(reg)
		return NewSaaSHandler(reg, billing, saas.NewProvisioningManager(reg, billing))
	})
	registerHandler("cost", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		tracker := cost.NewTracker("USD")
		return NewCostHandler(tracker, cost.NewBudgetManager(tracker), cost.NewChargebackManager(tracker))
	})
	registerHandler("cluster", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return nil
	})
	registerHandler("scheduler", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewSchedulerHandler(warehouse.NewCronScheduler(nil, slog.Default()))
	})
	registerHandler("lineage", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewLineageHandler(lineage.NewTracker())
	})
	registerHandler("wasm", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewWASMHandler(wasm.NewRuntime(wasm.DefaultConfig(), slog.Default()))
	})
	registerHandler("federation", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewFederationHandler(federation.NewFederation(federation.DefaultConfig()))
	})
	registerHandler("experiment", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewExperimentHandler(experiment.NewEngine())
	})
	registerHandler("dbt", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewDBTHandler(deps.Config.Dependencies.DBTOptions)
	})
	registerHandler("compute", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewComputeHandler(compute.NewComputeEngine(compute.DefaultComputeConfig()))
	})
	registerHandler("consensus", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		node := consensus.NewRaftNode(context.Background(), consensus.DefaultRaftConfig(), nil)
		return NewConsensusHandler(node, consensus.NewShardManager(16, node))
	})
	registerHandler("stream_sql", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewStreamSQLHandler(streamsql.NewEngine(streamsql.DefaultEngineConfig()))
	})
	registerHandler("control_plane", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewControlPlaneHandler(controlplane.NewManager(context.Background(), controlplane.DefaultManagerConfig()))
	})
	registerHandler("versioning", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewVersioningHandler(versioning.NewVersionStore())
	})
	registerHandler("validation", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		h := NewValidationHandler(validation.NewValidator(validation.DefaultValidatorConfig()))
		h.requireAuth = deps.AuthMiddleware
		return h
	})
	registerHandler("dashboard_v2", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewDashboardHandler(deps.Store, deps.Metrics)
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
	registerHandler("replication", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewReplicationHandler(replication.NewManager(replication.DefaultManagerConfig()))
	})
}

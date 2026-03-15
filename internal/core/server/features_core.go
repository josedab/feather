package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/feather-store/feather/internal/extensions/drift"
	"github.com/feather-store/feather/internal/extensions/freshness"
	"github.com/feather-store/feather/internal/extensions/semantic"
	"github.com/feather-store/feather/internal/extensions/sharding"
	"github.com/feather-store/feather/internal/platform/cluster"
	"github.com/feather-store/feather/internal/platform/quality"
	"github.com/feather-store/feather/internal/platform/sla"
)

// Handler registrations: Stable — Production-ready, well-tested, breaking changes
// follow semver. Safe for all deployments.
//
// Registers: groups, backfill, streaming, catalog, auth, ml, transform, cache,
// consistency, observability, benchmark, impact, model_serving, governance,
// freshness, sla, drift, semantic, quality, sharding, marketplace.
func init() {
	registerHandler("groups", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		h := NewGroupsHandler()
		h.requireAuth = deps.AuthMiddleware
		return h
	})
	registerHandler("backfill", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		h := NewBackfillHandler(deps.Store)
		h.requireAuth = deps.AuthMiddleware
		return h
	})
	registerHandler("streaming", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewStreamingHandler(deps.Ctx)
	})
	registerHandler("catalog", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		h := NewCatalogHandler()
		h.requireAuth = deps.AuthMiddleware
		return h
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
	registerHandler("governance", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewGovernanceHandler(GovernanceHandlerConfig{})
	})
	registerHandler("freshness", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewFreshnessHandler(freshness.NewManager(freshness.DefaultManagerConfig()))
	})
	registerHandler("sla", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewSLAHandler(sla.NewManager(nil, sla.DefaultManagerConfig()))
	})
	registerHandler("drift", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewDriftHandler(drift.NewDetector(drift.DefaultConfig()))
	})
	registerHandler("semantic", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewSemanticHandler(semantic.NewSearch(semantic.NewLocalEmbedder(128), slog.Default()))
	})
	registerHandler("quality", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewQualityHandler(quality.NewValidator())
	})
	registerHandler("sharding", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		ring := cluster.NewHashRing(150)
		ring.AddNode(&cluster.Node{ID: "local", Address: "localhost", Zone: "default", VirtualNodes: 150})
		router := sharding.NewRouter(sharding.RouterConfig{
			LocalNodeID:       "local",
			ReplicationFactor: 1,
			TotalPartitions:   256,
			WriteConsistency:  sharding.WriteConsistencyOne,
			ReadConsistency:   sharding.ReadConsistencyLocal,
			WriteTimeout:      5 * time.Second,
			ReadTimeout:       5 * time.Second,
		}, ring, &localReplicaClient{})
		return NewShardingHandler(router)
	})
	registerHandler("marketplace", MaturityStable, func(deps *handlerDeps) FeatureHandler {
		return NewMarketplaceHandler()
	})
}

// localReplicaClient is a no-op ReplicaClient for single-node mode.
type localReplicaClient struct{}

func (c *localReplicaClient) WriteFeature(_ context.Context, _ string, _ *sharding.WriteRequest) error {
	return fmt.Errorf("single-node mode: direct store access required")
}

func (c *localReplicaClient) ReadFeature(_ context.Context, _ string, _ *sharding.ReadRequest) (*sharding.ReadResponse, error) {
	return nil, fmt.Errorf("single-node mode: direct store access required")
}

package server

import (
	"github.com/feather-store/feather/internal/extensions/lifecycle"
	"github.com/feather-store/feather/internal/extensions/modelserving"
	"github.com/feather-store/feather/internal/extensions/semantic"
	"github.com/feather-store/feather/internal/extensions/starlarkudf"
	"github.com/feather-store/feather/internal/platform/federation"
)

// Handler registrations: Next-gen v5 — UDF runtime, GitOps, model serving,
// semantic discovery, lifecycle management, and federation enhancements.
//
// Registers: starlark_udf, model_gateway, semantic_catalog, federation_cross_org, lifecycle_manager.
func init() {
	registerHandler("starlark_udf", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewStarlarkUDFHandler(starlarkudf.NewRegistry(starlarkudf.DefaultRegistryConfig()))
	})
	registerHandler("model_gateway", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		reg := modelserving.NewRegistry(modelserving.DefaultRegistryConfig())
		gw := modelserving.NewGateway(reg)
		return NewModelGatewayHandler(gw)
	})
	registerHandler("semantic_catalog", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewSemanticCatalogHandler(semantic.NewCatalog(semantic.DefaultCatalogConfig()))
	})
	registerHandler("federation_cross_org", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewCrossOrgFederationHandler(federation.NewCrossOrgFederation(federation.DefaultCrossOrgConfig()))
	})
	registerHandler("lifecycle_manager", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewLifecycleManagerHandler(lifecycle.NewManager(lifecycle.DefaultManagerConfig()))
	})
}

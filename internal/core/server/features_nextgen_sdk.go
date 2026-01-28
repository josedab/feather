package server

import (
	"github.com/feather-store/feather/internal/extensions/consistencyvalidator"
	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
	"github.com/feather-store/feather/internal/extensions/feastcompat"
	"github.com/feather-store/feather/internal/extensions/featherqlv2"
	"github.com/feather-store/feather/internal/extensions/featuredashboard"
	"github.com/feather-store/feather/internal/extensions/promptstore"
	"github.com/feather-store/feather/internal/extensions/saascontrol"
	"github.com/feather-store/feather/internal/extensions/schemaevolution"
	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
	"github.com/feather-store/feather/internal/extensions/streamcompute"
)

// Handler registrations: Next-gen v1 — SDK, pipeline, and compatibility features.

func init() {
	registerHandler("stream_compute", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewStreamComputeHandler(streamcompute.NewEngine(streamcompute.DefaultEngineConfig()))
	})
	registerHandler("sdk_codegen", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewSDKCodegenHandler(sdkcodegen.NewGenerator(sdkcodegen.DefaultGeneratorConfig()))
	})
	registerHandler("prompt_store", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewPromptStoreHandler(promptstore.NewStore(promptstore.DefaultStoreConfig()))
	})
	registerHandler("consistency_validator", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewConsistencyValidatorHandler(consistencyvalidator.NewValidator(consistencyvalidator.DefaultValidatorConfig()))
	})
	registerHandler("feature_dashboard", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewFeatureDashboardHandler(featuredashboard.NewDashboard(featuredashboard.DefaultDashboardConfig()))
	})
	registerHandler("featherql_v2", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewFeatherQLv2Handler(featherqlv2.NewEngine(featherqlv2.DefaultEngineConfig()))
	})
	registerHandler("embedding_mgmt", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewEmbeddingMgmtHandler(embeddingmgmt.NewManager(embeddingmgmt.DefaultManagerConfig()))
	})
	registerHandler("schema_evolution", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewSchemaEvolutionHandler(schemaevolution.NewManager(schemaevolution.DefaultManagerConfig()))
	})
	registerHandler("feast_compat", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewFeastCompatHandler(feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig()))
	})
	registerHandler("feast_gateway", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		adapter := feastcompat.NewAdapter(feastcompat.DefaultAdapterConfig())
		return NewFeastGatewayHandler(feastcompat.NewGateway(adapter))
	})
	registerHandler("saas_control", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewSaaSControlHandler(saascontrol.NewControlPlane(saascontrol.DefaultControlPlaneConfig()))
	})
}

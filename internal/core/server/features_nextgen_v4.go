package server

import (
	"github.com/feather-store/feather/internal/extensions/arrowflight"
	"github.com/feather-store/feather/internal/extensions/audittrail"
	"github.com/feather-store/feather/internal/extensions/compression"
	"github.com/feather-store/feather/internal/extensions/diffprivacy"
	"github.com/feather-store/feather/internal/extensions/fedlearning"
	"github.com/feather-store/feather/internal/extensions/notebooksdk"
	"github.com/feather-store/feather/internal/extensions/playgroundv2"
	"github.com/feather-store/feather/internal/extensions/prefetch"
	"github.com/feather-store/feather/internal/extensions/qualitygates"
	"github.com/feather-store/feather/internal/extensions/queryplanner"
)

// Handler registrations: Next-gen v4 — Advanced data transport, privacy, ML optimization,
// and developer experience features.

func init() {
	registerHandler("arrow_flight", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewArrowFlightHandler(arrowflight.NewServer(arrowflight.DefaultConfig()))
	})
	registerHandler("diff_privacy", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewDiffPrivacyHandler(diffprivacy.NewEngine(diffprivacy.DefaultConfig()))
	})
	registerHandler("prefetch", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPrefetchHandler(prefetch.NewController(prefetch.DefaultConfig()))
	})
	registerHandler("notebook_sdk", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewNotebookSDKHandler(notebooksdk.NewService(notebooksdk.DefaultConfig()))
	})
	registerHandler("quality_gates", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewQualityGatesHandler(qualitygates.NewValidator(qualitygates.DefaultConfig()))
	})
	registerHandler("compression", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewCompressionHandler(compression.NewSelector(compression.DefaultConfig()))
	})
	registerHandler("audit_trail", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewAuditTrailHandler(audittrail.New(audittrail.DefaultConfig()))
	})
	registerHandler("query_planner", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewQueryPlannerHandler(queryplanner.New(queryplanner.DefaultConfig()))
	})
	registerHandler("fed_learning", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewFedLearningHandler(fedlearning.NewAdapter(fedlearning.DefaultConfig()))
	})
	registerHandler("playground_v2", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewPlaygroundV2Handler(playgroundv2.NewEnvironment(playgroundv2.DefaultConfig(), nil))
	})
}

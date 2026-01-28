package server

import (
	"github.com/feather-store/feather/internal/extensions/anomalydetect"
	"github.com/feather-store/feather/internal/extensions/apigateway"
	"github.com/feather-store/feather/internal/extensions/auditlog"
	"github.com/feather-store/feather/internal/extensions/cloudstorage"
	"github.com/feather-store/feather/internal/extensions/feathercli"
	"github.com/feather-store/feather/internal/extensions/importancescoring"
	"github.com/feather-store/feather/internal/extensions/incrmat"
	"github.com/feather-store/feather/internal/extensions/openapisync"
	"github.com/feather-store/feather/internal/extensions/terraformprovider"
	"github.com/feather-store/feather/internal/extensions/webhooks"
)

// Handler registrations: Next-gen v3 — Operations, governance, and infrastructure features.

func init() {
	registerHandler("api_gateway", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewAPIGatewayHandler(apigateway.NewGateway(apigateway.DefaultGatewayConfig()))
	})
	registerHandler("importance_scoring", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewImportanceScoringHandler(importancescoring.NewScorer(importancescoring.DefaultScorerConfig()))
	})
	registerHandler("anomaly_detect", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewAnomalyDetectHandler(anomalydetect.NewDetector(anomalydetect.DefaultDetectorConfig()))
	})
	registerHandler("feather_cli", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewFeatherCLIHandler(feathercli.NewClient(feathercli.DefaultClientConfig()))
	})
	registerHandler("incr_materialization", MaturityBeta, func(deps *handlerDeps) FeatureHandler {
		return NewIncrMatHandler(incrmat.NewEngine(incrmat.DefaultEngineConfig()))
	})
	registerHandler("webhook_events", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewWebhooksHandler(webhooks.NewDispatcher(webhooks.DefaultDispatcherConfig()))
	})
	registerHandler("cloud_storage", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewCloudStorageHandler(cloudstorage.NewObjectStore(cloudstorage.DefaultStoreConfig()))
	})
	registerHandler("audit_log", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewAuditLogHandler(auditlog.NewLogger(auditlog.DefaultLoggerConfig()))
	})
	registerHandler("openapi_sync", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewOpenAPISyncHandler(openapisync.NewGenerator(openapisync.DefaultGeneratorConfig()))
	})
	registerHandler("terraform_provider", MaturityExperimental, func(deps *handlerDeps) FeatureHandler {
		return NewTerraformProviderHandler(terraformprovider.NewProvider(terraformprovider.DefaultProviderConfig()))
	})
}

// Package kubeflow provides integration with Kubeflow Pipelines for ML
// feature engineering workflows.
//
// It manages pipeline components for feature computation, supports
// auto-registration of feature outputs, and integrates with Kubeflow's
// component-based pipeline model. Configure via [Config] with namespace
// and pipeline host settings.
//
// Usage:
//
//	manager := kubeflow.NewPipelineManager(kubeflow.Config{
//	    Namespace:    "ml-pipelines",
//	    PipelineHost: "http://localhost:8888",
//	    AutoRegister: true,
//	})
//	manager.RegisterComponent(ctx, component)
package kubeflow

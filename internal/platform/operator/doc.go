// Package operator provides a Kubernetes operator for managing Feather feature stores.
//
// It implements custom resource definitions (CRDs) for declarative feature store management:
//   - FeatureStore: Main resource for deploying feature store instances
//   - FeatureGroup: Defines feature groups and their schemas
//   - FeatureSource: Defines data sources for feature ingestion
//   - FeatureView: Defines materialized views over features
//
// The operator handles:
//   - Automatic scaling based on load
//   - Rolling updates with zero downtime
//   - Health monitoring and self-healing
//   - Resource quota management
//   - Integration with Kubernetes secrets for credentials
//
// # Usage
//
//	// Create controller manager
//	mgr, err := operator.NewManager(operator.ManagerConfig{
//	    MetricsAddr:   ":8081",
//	    ProbeAddr:     ":8082",
//	    LeaderElect:   true,
//	    Namespace:     "feather-system",
//	})
//
//	// Start the manager
//	mgr.Start(ctx)
package operator

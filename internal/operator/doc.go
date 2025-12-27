// Package operator provides a Kubernetes operator for the feature store.
//
// It implements custom resource definitions (CRDs) and controllers for
// deploying and managing Feather instances on Kubernetes. The operator
// handles scaling, upgrades, and configuration management.
//
// Custom Resources:
//   - FeatherStore: Defines a Feather deployment with replicas and configuration
package operator

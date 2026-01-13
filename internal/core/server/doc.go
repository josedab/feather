// Package server provides HTTP and gRPC servers for the feature store.
//
// It implements the feature store API including feature retrieval, storage,
// batch operations, point-in-time queries, and schema management. The package
// includes health checks, middleware for authentication and rate limiting,
// and integration with observability systems.
//
// Key components:
//   - HTTPServer: REST API server with versioned endpoints
//   - GRPCServer: High-performance gRPC server with streaming support
//   - HealthChecker: Kubernetes-compatible health and readiness probes
package server

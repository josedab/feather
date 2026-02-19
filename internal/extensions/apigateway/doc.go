// Package apigateway provides a smart API gateway with request coalescing,
// multi-instance load balancing, and per-tenant rate limiting for Feather
// feature store deployments.
//
// Key components:
//   - Gateway: Routes requests across backends with health-aware load balancing
//   - Backend: Represents a downstream Feather instance with health tracking
//   - CoalesceWindow: Groups duplicate entity key lookups within a time window
package apigateway

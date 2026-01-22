// Package saascontrol provides a multi-tenant SaaS control plane for
// provisioning, scaling, monitoring, and billing isolated Feather instances.
// It includes usage metering, quota enforcement, and self-service management.
//
// Key components:
//   - ControlPlane: Manages tenant lifecycle and instance provisioning
//   - Tenant: Represents an isolated customer environment
//   - UsageMeter: Tracks per-tenant resource consumption
package saascontrol

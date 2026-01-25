// Package cloudcontrol provides a managed cloud control plane for provisioning,
// scaling, and managing Feather instances. It handles instance lifecycle,
// autoscaling policies, multi-tenant isolation, and resource quota management.
//
// Key components:
//   - ControlPlane: Orchestrates instance lifecycle and tenant management
//   - Instance: Represents a managed Feather deployment
//   - AutoscalePolicy: Defines autoscaling rules for instances
//   - Tenant: Isolated customer environment with quotas
package cloudcontrol

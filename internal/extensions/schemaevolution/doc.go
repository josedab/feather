// Package schemaevolution provides zero-downtime schema migration with
// backward/forward compatibility checking, automatic type coercion,
// and blue-green schema deployments.
//
// Key components:
//   - Manager: Orchestrates schema migrations
//   - CompatibilityChecker: Validates schema compatibility
//   - Migration: Represents a schema change with rollback capability
package schemaevolution

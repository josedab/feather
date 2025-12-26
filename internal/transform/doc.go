// Package transform provides feature transformation pipelines.
//
// It enables defining and executing transformations that derive new features
// from existing ones. The package supports a DSL for expressing transformations,
// automatic dependency resolution, and caching of computed results.
//
// Key components:
//   - Pipeline: Defines a sequence of feature transformations
//   - Executor: Runs transformation pipelines with dependency tracking
//   - DSL: Domain-specific language for defining transformations
package transform

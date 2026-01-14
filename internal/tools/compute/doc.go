// Package compute provides a Feature Computation Engine (FCE) for the Feather
// feature store.
//
// The FCE enables declarative feature definitions with a DSL, scheduled
// materialization of computed features, and incremental computation that
// only recomputes when inputs change. It builds on the existing transform
// pipeline and composition engine primitives.
//
// # Feature Definitions
//
// Features are defined declaratively with expressions, input dependencies,
// and compute modes:
//
//	def := &compute.FeatureDefinition{
//	    Name:       "user_score",
//	    Expression: "purchase_total * 0.5 + click_count * 0.1",
//	    Inputs:     []string{"purchase_total", "click_count"},
//	    OutputType: "float64",
//	    Mode:       compute.ComputeModeOnDemand,
//	}
//	engine.Define(ctx, def)
//
// # Compute Modes
//
// Four compute modes are supported:
//   - OnDemand: computed when requested
//   - Scheduled: computed on a cron schedule
//   - Streaming: computed as inputs arrive
//   - Batch: computed in bulk over entity sets
//
// # Incremental Computation
//
// When Incremental is set on a definition, the engine caches results and
// only recomputes when input values change, providing significant performance
// improvements for expensive computations.
package compute

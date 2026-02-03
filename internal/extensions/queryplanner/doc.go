// Package queryplanner provides a cost-based query optimizer for the composition
// engine and FeatherQL with adaptive replanning.
//
// # Usage
//
//	planner := queryplanner.NewPlanner(queryplanner.DefaultConfig())
//	plan, _ := planner.Optimize(query)
//	result, _ := plan.Execute(ctx)
package queryplanner

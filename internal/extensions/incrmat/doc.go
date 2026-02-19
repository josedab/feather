// Package incrmat provides incremental materialization driven by
// change-data-capture, recomputing only features whose upstream data
// has changed with dependency-aware scheduling.
//
// Key components:
//   - Engine: Manages a DAG of materialization nodes and processes changes
//   - MaterializationNode: Represents a feature computation with dependencies
//   - ChangeEvent: Captures upstream data mutations for incremental processing
package incrmat

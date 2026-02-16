// Package streamcompute provides a built-in stateful stream processing engine
// for computing features from event streams without requiring external systems
// like Flink or Spark. It supports tumbling, sliding, and session windows with
// exactly-once semantics and pluggable state backends.
//
// Key components:
//   - Engine: Orchestrates stream processing pipelines
//   - Window: Configurable windowing strategies (tumbling, sliding, session)
//   - Pipeline: Defines event sources, transformations, and sinks
package streamcompute

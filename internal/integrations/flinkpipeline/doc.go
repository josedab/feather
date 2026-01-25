// Package flinkpipeline provides streaming feature pipelines with Flink and
// Kafka Streams integration for real-time feature computation. It enables
// continuous feature transformation from event streams with support for
// tumbling, sliding, and session windows.
//
// Key components:
//   - Manager: Orchestrates streaming pipeline lifecycle
//   - Pipeline: Defines a streaming feature computation graph
//   - Source: Kafka topic or event stream source
//   - Sink: Feature store sink for computed features
package flinkpipeline

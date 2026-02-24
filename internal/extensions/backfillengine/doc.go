// Package backfillengine provides a unified streaming backfill engine that
// automatically replays and backfills features from streaming sources (Kafka,
// Flink) into warm/historical tiers with exactly-once semantics and
// point-in-time correctness.
//
// The engine unifies online and offline feature pipelines, eliminating
// training-serving skew by ensuring the same computation logic runs for both
// real-time and historical data.
//
// # Architecture
//
// The engine is composed of three layers:
//
//   - Source Abstraction Layer: Pluggable source readers (Kafka, Flink, file)
//     with unified event format and offset management.
//   - Backfill Coordinator: Orchestrates backfill jobs with exactly-once
//     semantics, checkpointing, and parallelism control.
//   - Point-in-Time Materializer: Writes features with correct timestamps
//     into the warm tier, preserving temporal ordering for accurate
//     historical queries.
//
// # Usage
//
//	coordinator := backfillengine.NewCoordinator(backfillengine.DefaultCoordinatorConfig())
//	source := backfillengine.NewKafkaSource(backfillengine.KafkaSourceConfig{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:   "features",
//	})
//	coordinator.RegisterSource("kafka-features", source)
//	job, _ := coordinator.CreateJob(backfillengine.JobRequest{
//	    SourceName: "kafka-features",
//	    StartTime:  time.Now().Add(-24 * time.Hour),
//	    EndTime:    time.Now(),
//	})
//	coordinator.StartJob(job.ID)
package backfillengine

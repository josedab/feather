// Package ingestion provides data ingestion pipelines for the feature store.
//
// It supports multiple ingestion sources including HTTP push endpoints and
// Kafka consumers. The package implements rate limiting, circuit breakers,
// and batch processing for reliable high-throughput data ingestion.
//
// Key components:
//   - HTTPIngestion: REST API for pushing feature updates
//   - KafkaConsumer: Consumes feature updates from Kafka topics
//   - CircuitBreaker: Provides fault tolerance for ingestion pipelines
package ingestion

// Package backfill provides historical feature data backfilling capabilities.
//
// It supports importing historical feature values from external sources into
// the feature store, enabling point-in-time feature retrieval for training
// data generation. The package handles large-scale batch imports efficiently
// with progress tracking and resumable operations.
//
// Key components:
//   - Manager: Orchestrates backfill jobs and tracks progress
//   - Job: Represents a single backfill operation with source and destination
package backfill

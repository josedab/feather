// Package offlinestore provides a Parquet-based offline store backend for
// seamless training data export, enabling point-in-time correct feature
// retrieval for model training.
//
// Key components:
//   - Store: Manages datasets and feature rows for offline access
//   - DatasetConfig: Defines the parameters for a dataset
//   - FeatureRow: Represents a single feature observation with timestamp
package offlinestore

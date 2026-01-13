// Package domain defines the core domain types for the feature store.
//
// It contains the fundamental data structures including FeatureValue, FeatureGroup,
// FeatureSpec, and aggregation specifications. The package also defines standard
// errors and data type enumerations used throughout the system.
//
// Key types:
//   - FeatureValue: Represents a stored feature with value, timestamp, and version
//   - FeatureGroup: Defines a collection of related features with shared properties
//   - FeatureSpec: Specifies the schema for an individual feature
//   - AggregationSpec: Configures real-time aggregation behavior
package domain

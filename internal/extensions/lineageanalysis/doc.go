// Package lineageanalysis provides feature lineage tracking and impact analysis
// for tracing features from source data through transformations to serving.
// It enables impact analysis when upstream data changes, helping teams understand
// feature dependencies and blast radius of changes.
//
// Key components:
//   - Tracker: Manages the lineage graph and dependency tracking
//   - LineageNode: Represents a data source, transformation, or feature
//   - ImpactReport: Shows downstream effects of upstream changes
package lineageanalysis

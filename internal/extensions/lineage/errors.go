package lineage

import "errors"

var (
	// ErrFeatureNotFound is returned when a feature doesn't exist.
	ErrFeatureNotFound = errors.New("feature not found")

	// ErrSourceNotFound is returned when a source doesn't exist.
	ErrSourceNotFound = errors.New("source not found")

	// ErrConsumerNotFound is returned when a consumer doesn't exist.
	ErrConsumerNotFound = errors.New("consumer not found")

	// ErrCycleDetected is returned when a dependency cycle is found.
	ErrCycleDetected = errors.New("dependency cycle detected")

	// ErrInvalidLineage is returned when lineage data is invalid.
	ErrInvalidLineage = errors.New("invalid lineage data")
)

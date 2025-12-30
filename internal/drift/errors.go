package drift

import "errors"

var (
	// ErrFeatureNotFound is returned when a monitored feature doesn't exist.
	ErrFeatureNotFound = errors.New("feature not found")
)

package featuredashboard

import "errors"

var (
	// ErrFeatureNotTracked is returned when a feature is not being tracked.
	ErrFeatureNotTracked = errors.New("feature not tracked")
)

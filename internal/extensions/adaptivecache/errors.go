package adaptivecache

import "errors"

var (
	// ErrFeatureNotTracked is returned when a feature key is not being tracked.
	ErrFeatureNotTracked = errors.New("feature not tracked")
)

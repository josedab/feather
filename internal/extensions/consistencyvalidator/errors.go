package consistencyvalidator

import "errors"

var (
	// ErrFeatureNotRegistered is returned when a feature is not being monitored.
	ErrFeatureNotRegistered = errors.New("feature not registered for consistency monitoring")

	// ErrInsufficientData is returned when there isn't enough data for comparison.
	ErrInsufficientData = errors.New("insufficient data for consistency check")
)

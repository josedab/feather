package backpressure

import "errors"

var (
	// ErrMetricNotFound is returned when a requested metric doesn't exist.
	ErrMetricNotFound = errors.New("metric not found")

	// ErrInvalidThreshold is returned when a threshold value is invalid.
	ErrInvalidThreshold = errors.New("invalid threshold")
)

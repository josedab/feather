package offlinestore

import "errors"

var (
	// ErrDatasetNotFound is returned when a dataset doesn't exist.
	ErrDatasetNotFound = errors.New("dataset not found")

	// ErrDatasetExists is returned when a dataset already exists.
	ErrDatasetExists = errors.New("dataset already exists")

	// ErrInvalidTimeRange is returned when a time range is invalid.
	ErrInvalidTimeRange = errors.New("invalid time range")
)

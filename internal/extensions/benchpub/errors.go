package benchpub

import "errors"

var (
	// ErrBenchmarkNotFound is returned when a benchmark result doesn't exist.
	ErrBenchmarkNotFound = errors.New("benchmark not found")

	// ErrBenchmarkRunning is returned when a benchmark is already in progress.
	ErrBenchmarkRunning = errors.New("benchmark already running")
)

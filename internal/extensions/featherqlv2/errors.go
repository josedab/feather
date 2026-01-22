package featherqlv2

import "errors"

var (
	// ErrParseFailed is returned when query parsing fails.
	ErrParseFailed = errors.New("parse failed")

	// ErrCompileFailed is returned when query compilation fails.
	ErrCompileFailed = errors.New("compilation failed")

	// ErrExecutionFailed is returned when query execution fails.
	ErrExecutionFailed = errors.New("execution failed")

	// ErrPipelineNotFound is returned when a compiled pipeline doesn't exist.
	ErrPipelineNotFound = errors.New("pipeline not found")
)

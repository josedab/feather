package streamcompute

import "errors"

var (
	// ErrPipelineNotFound is returned when a pipeline does not exist.
	ErrPipelineNotFound = errors.New("pipeline not found")

	// ErrPipelineExists is returned when a pipeline with the same ID already exists.
	ErrPipelineExists = errors.New("pipeline already exists")

	// ErrInvalidWindow is returned when a window configuration is invalid.
	ErrInvalidWindow = errors.New("invalid window configuration")

	// ErrPipelineRunning is returned when attempting to modify a running pipeline.
	ErrPipelineRunning = errors.New("pipeline is running")

	// ErrPipelineStopped is returned when attempting to operate on a stopped pipeline.
	ErrPipelineStopped = errors.New("pipeline is stopped")
)

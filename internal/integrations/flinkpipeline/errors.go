package flinkpipeline

import "errors"

var (
	// ErrPipelineNotFound is returned when a pipeline does not exist.
	ErrPipelineNotFound = errors.New("pipeline not found")

	// ErrPipelineExists is returned when a pipeline with the same ID already exists.
	ErrPipelineExists = errors.New("pipeline already exists")

	// ErrInvalidPipeline is returned when pipeline configuration is invalid.
	ErrInvalidPipeline = errors.New("invalid pipeline configuration")

	// ErrPipelineRunning is returned when trying to modify a running pipeline.
	ErrPipelineRunning = errors.New("pipeline is currently running")

	// ErrSourceNotConfigured is returned when a pipeline has no source.
	ErrSourceNotConfigured = errors.New("source not configured")
)

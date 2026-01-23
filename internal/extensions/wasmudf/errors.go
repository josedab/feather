package wasmudf

import "errors"

var (
	// ErrModuleNotFound is returned when a WASM module doesn't exist.
	ErrModuleNotFound = errors.New("module not found")

	// ErrModuleExists is returned when a module with the same ID already exists.
	ErrModuleExists = errors.New("module already exists")

	// ErrExecutionFailed is returned when module execution fails.
	ErrExecutionFailed = errors.New("execution failed")

	// ErrResourceLimit is returned when a resource limit is exceeded.
	ErrResourceLimit = errors.New("resource limit exceeded")
)

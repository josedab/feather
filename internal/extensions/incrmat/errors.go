package incrmat

import "errors"

var (
	// ErrNodeNotRegistered is returned when referencing a node that does not exist.
	ErrNodeNotRegistered = errors.New("node not registered")

	// ErrCyclicDependency is returned when adding a node would create a cycle.
	ErrCyclicDependency = errors.New("cyclic dependency detected")

	// ErrMaterializationFailed is returned when a materialization step fails.
	ErrMaterializationFailed = errors.New("materialization failed")
)

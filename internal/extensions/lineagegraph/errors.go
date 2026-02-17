package lineagegraph

import "errors"

var (
	// ErrNodeNotFound is returned when a node does not exist in the graph.
	ErrNodeNotFound = errors.New("node not found")

	// ErrNodeExists is returned when a node already exists.
	ErrNodeExists = errors.New("node already exists")

	// ErrEdgeExists is returned when an edge already exists.
	ErrEdgeExists = errors.New("edge already exists")

	// ErrCyclicDependency is returned when adding an edge would create a cycle.
	ErrCyclicDependency = errors.New("cyclic dependency detected")
)

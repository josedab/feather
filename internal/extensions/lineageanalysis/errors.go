package lineageanalysis

import "errors"

var (
	ErrNodeNotFound   = errors.New("lineage node not found")
	ErrNodeExists     = errors.New("lineage node already exists")
	ErrEdgeExists     = errors.New("lineage edge already exists")
	ErrCyclicLineage  = errors.New("would create a cycle in lineage graph")
	ErrInvalidNode    = errors.New("invalid node configuration")
)

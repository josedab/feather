package vector

import "errors"

var (
	// ErrDimensionMismatch is returned when vector dimensions don't match.
	ErrDimensionMismatch = errors.New("vector dimension mismatch")

	// ErrIndexNotFound is returned when an index doesn't exist.
	ErrIndexNotFound = errors.New("vector index not found")

	// ErrVectorNotFound is returned when a vector doesn't exist.
	ErrVectorNotFound = errors.New("vector not found")

	// ErrInvalidDistance is returned for invalid distance type.
	ErrInvalidDistance = errors.New("invalid distance type")
)

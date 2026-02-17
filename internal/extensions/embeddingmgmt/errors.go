package embeddingmgmt

import "errors"

var (
	// ErrModelNotFound is returned when an embedding model is not registered.
	ErrModelNotFound = errors.New("model not found")

	// ErrCollectionNotFound is returned when an embedding collection doesn't exist.
	ErrCollectionNotFound = errors.New("collection not found")

	// ErrCollectionExists is returned when a collection already exists.
	ErrCollectionExists = errors.New("collection already exists")

	// ErrDimensionMismatch is returned when embedding dimensions don't match.
	ErrDimensionMismatch = errors.New("dimension mismatch")
)

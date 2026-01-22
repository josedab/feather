package feastcompat

import "errors"

var (
	// ErrFeatureViewNotFound is returned when a Feast feature view mapping doesn't exist.
	ErrFeatureViewNotFound = errors.New("feature view not found")

	// ErrEntityNotFound is returned when an entity doesn't exist.
	ErrEntityNotFound = errors.New("entity not found")

	// ErrMappingExists is returned when a mapping already exists.
	ErrMappingExists = errors.New("mapping already exists")
)

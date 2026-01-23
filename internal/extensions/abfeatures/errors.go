package abfeatures

import "errors"

var (
	// ErrExperimentNotFound is returned when an experiment doesn't exist.
	ErrExperimentNotFound = errors.New("experiment not found")

	// ErrExperimentExists is returned when an experiment already exists.
	ErrExperimentExists = errors.New("experiment already exists")

	// ErrVariantNotFound is returned when a variant doesn't exist.
	ErrVariantNotFound = errors.New("variant not found")
)

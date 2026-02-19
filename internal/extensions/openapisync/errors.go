package openapisync

import "errors"

var (
	// ErrSpecNotGenerated is returned when the OpenAPI spec has not been generated yet.
	ErrSpecNotGenerated = errors.New("spec not generated")

	// ErrInvalidRoute is returned when a route definition is invalid.
	ErrInvalidRoute = errors.New("invalid route")
)

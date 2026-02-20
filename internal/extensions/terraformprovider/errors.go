package terraformprovider

import "errors"

var (
	// ErrResourceNotFound is returned when the requested resource does not exist.
	ErrResourceNotFound = errors.New("resource not found")

	// ErrResourceExists is returned when a resource with the same ID already exists.
	ErrResourceExists = errors.New("resource already exists")

	// ErrInvalidResourceType is returned when the resource type is not recognized.
	ErrInvalidResourceType = errors.New("invalid resource type")
)

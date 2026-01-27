package pythonsdk

import "errors"

var (
	ErrTransformNotFound = errors.New("transform not found")
	ErrTransformExists   = errors.New("transform already exists")
	ErrInvalidTransform  = errors.New("invalid transform definition")
	ErrWorkerUnavailable = errors.New("python worker unavailable")
)

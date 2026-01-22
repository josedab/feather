package promptstore

import "errors"

var (
	// ErrPromptNotFound is returned when a prompt template does not exist.
	ErrPromptNotFound = errors.New("prompt not found")

	// ErrPromptExists is returned when a prompt with the same ID already exists.
	ErrPromptExists = errors.New("prompt already exists")

	// ErrVersionNotFound is returned when a specific version does not exist.
	ErrVersionNotFound = errors.New("version not found")

	// ErrInvalidTemplate is returned when the prompt template is malformed.
	ErrInvalidTemplate = errors.New("invalid template")
)

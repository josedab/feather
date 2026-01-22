package sdkcodegen

import "errors"

var (
	// ErrInvalidSchema is returned when the input schema is invalid.
	ErrInvalidSchema = errors.New("invalid schema")

	// ErrUnsupportedLanguage is returned for unsupported target languages.
	ErrUnsupportedLanguage = errors.New("unsupported language")

	// ErrGenerationFailed is returned when code generation fails.
	ErrGenerationFailed = errors.New("generation failed")
)

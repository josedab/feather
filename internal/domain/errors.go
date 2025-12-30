package domain

import (
	"errors"
	"fmt"
)

// Error codes for API responses.
const (
	// Client errors (4xx)
	ErrCodeBadRequest       = "BAD_REQUEST"
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeValidationFailed = "VALIDATION_FAILED"
	ErrCodeRateLimited      = "RATE_LIMITED"
	ErrCodeRequestTooLarge  = "REQUEST_TOO_LARGE"

	// Server errors (5xx)
	ErrCodeInternal           = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeStorageFull        = "STORAGE_FULL"
	ErrCodeTimeout            = "TIMEOUT"
)

// Sentinel errors for common cases.
var (
	ErrEntityNotFound  = errors.New("entity not found")
	ErrFeatureNotFound = errors.New("feature not found")
	ErrGroupNotFound   = errors.New("feature group not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrTypeMismatch    = errors.New("type mismatch")
	ErrValidation      = errors.New("validation failed")
	ErrStorageFull     = errors.New("storage full")
	ErrAlreadyExists   = errors.New("already exists")
	ErrRateLimited     = errors.New("rate limited")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrTimeout         = errors.New("operation timed out")
)

// ValidationError represents a validation failure with field details.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// APIError represents a structured API error with code and details.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewAPIError creates a new API error.
func NewAPIError(code, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
	}
}

// WithDetails adds details to the API error.
func (e *APIError) WithDetails(details map[string]string) *APIError {
	e.Details = details
	return e
}

// WrapError wraps an error with additional context.
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrEntityNotFound) ||
		errors.Is(err, ErrFeatureNotFound) ||
		errors.Is(err, ErrGroupNotFound)
}

// IsValidation checks if the error is a validation error.
func IsValidation(err error) bool {
	if errors.Is(err, ErrValidation) || errors.Is(err, ErrInvalidInput) {
		return true
	}
	var ve *ValidationError
	return errors.As(err, &ve)
}

// IsConflict checks if the error is a conflict error.
func IsConflict(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsRateLimited checks if the error is a rate limit error.
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsUnauthorized checks if the error is an authorization error.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsForbidden checks if the error is a forbidden error.
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// ErrorToCode converts an error to an API error code.
func ErrorToCode(err error) string {
	switch {
	case IsNotFound(err):
		return ErrCodeNotFound
	case IsValidation(err):
		return ErrCodeValidationFailed
	case IsConflict(err):
		return ErrCodeConflict
	case IsRateLimited(err):
		return ErrCodeRateLimited
	case IsUnauthorized(err):
		return ErrCodeUnauthorized
	case IsForbidden(err):
		return ErrCodeForbidden
	case errors.Is(err, ErrStorageFull):
		return ErrCodeStorageFull
	case errors.Is(err, ErrTimeout):
		return ErrCodeTimeout
	default:
		return ErrCodeInternal
	}
}

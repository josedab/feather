package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "email",
		Message: "invalid format",
	}

	expected := "validation error on email: invalid format"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		Code:    ErrCodeBadRequest,
		Message: "missing required field",
	}

	expected := "BAD_REQUEST: missing required field"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestNewAPIError(t *testing.T) {
	err := NewAPIError(ErrCodeNotFound, "entity not found")

	if err.Code != ErrCodeNotFound {
		t.Errorf("Expected code %q, got %q", ErrCodeNotFound, err.Code)
	}
	if err.Message != "entity not found" {
		t.Errorf("Expected message %q, got %q", "entity not found", err.Message)
	}
	if err.Details != nil {
		t.Error("Expected nil details")
	}
}

func TestAPIError_WithDetails(t *testing.T) {
	err := NewAPIError(ErrCodeValidationFailed, "validation failed").
		WithDetails(map[string]string{
			"field": "email",
			"error": "invalid format",
		})

	if len(err.Details) != 2 {
		t.Errorf("Expected 2 details, got %d", len(err.Details))
	}
	if err.Details["field"] != "email" {
		t.Errorf("Expected field=email, got %q", err.Details["field"])
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		msg      string
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			msg:      "context",
			expected: "",
		},
		{
			name:     "with error",
			err:      errors.New("original"),
			msg:      "context",
			expected: "context: original",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapError(tt.err, tt.msg)
			if tt.err == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
			} else {
				if result.Error() != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result.Error())
				}
			}
		})
	}
}

func TestWrapError_Unwrap(t *testing.T) {
	original := ErrEntityNotFound
	wrapped := WrapError(original, "getting user")

	if !errors.Is(wrapped, ErrEntityNotFound) {
		t.Error("Expected wrapped error to match ErrEntityNotFound")
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrEntityNotFound", ErrEntityNotFound, true},
		{"ErrFeatureNotFound", ErrFeatureNotFound, true},
		{"ErrGroupNotFound", ErrGroupNotFound, true},
		{"wrapped ErrEntityNotFound", fmt.Errorf("context: %w", ErrEntityNotFound), true},
		{"ErrValidation", ErrValidation, false},
		{"generic error", errors.New("some error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.expected {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsValidation(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrValidation", ErrValidation, true},
		{"ErrInvalidInput", ErrInvalidInput, true},
		{"ValidationError", &ValidationError{Field: "test", Message: "error"}, true},
		{"wrapped ErrValidation", fmt.Errorf("context: %w", ErrValidation), true},
		{"wrapped ValidationError", fmt.Errorf("context: %w", &ValidationError{Field: "test", Message: "error"}), true},
		{"ErrEntityNotFound", ErrEntityNotFound, false},
		{"generic error", errors.New("some error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidation(tt.err); got != tt.expected {
				t.Errorf("IsValidation(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrAlreadyExists", ErrAlreadyExists, true},
		{"wrapped ErrAlreadyExists", fmt.Errorf("context: %w", ErrAlreadyExists), true},
		{"ErrEntityNotFound", ErrEntityNotFound, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConflict(tt.err); got != tt.expected {
				t.Errorf("IsConflict(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrRateLimited", ErrRateLimited, true},
		{"wrapped ErrRateLimited", fmt.Errorf("context: %w", ErrRateLimited), true},
		{"ErrEntityNotFound", ErrEntityNotFound, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRateLimited(tt.err); got != tt.expected {
				t.Errorf("IsRateLimited(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsUnauthorized(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrUnauthorized", ErrUnauthorized, true},
		{"wrapped ErrUnauthorized", fmt.Errorf("context: %w", ErrUnauthorized), true},
		{"ErrEntityNotFound", ErrEntityNotFound, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnauthorized(tt.err); got != tt.expected {
				t.Errorf("IsUnauthorized(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsForbidden(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrForbidden", ErrForbidden, true},
		{"wrapped ErrForbidden", fmt.Errorf("context: %w", ErrForbidden), true},
		{"ErrUnauthorized", ErrUnauthorized, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForbidden(tt.err); got != tt.expected {
				t.Errorf("IsForbidden(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestErrorToCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrEntityNotFound", ErrEntityNotFound, ErrCodeNotFound},
		{"ErrFeatureNotFound", ErrFeatureNotFound, ErrCodeNotFound},
		{"ErrGroupNotFound", ErrGroupNotFound, ErrCodeNotFound},
		{"ErrValidation", ErrValidation, ErrCodeValidationFailed},
		{"ErrInvalidInput", ErrInvalidInput, ErrCodeValidationFailed},
		{"ValidationError", &ValidationError{Field: "test", Message: "error"}, ErrCodeValidationFailed},
		{"ErrAlreadyExists", ErrAlreadyExists, ErrCodeConflict},
		{"ErrRateLimited", ErrRateLimited, ErrCodeRateLimited},
		{"ErrUnauthorized", ErrUnauthorized, ErrCodeUnauthorized},
		{"ErrForbidden", ErrForbidden, ErrCodeForbidden},
		{"ErrStorageFull", ErrStorageFull, ErrCodeStorageFull},
		{"ErrTimeout", ErrTimeout, ErrCodeTimeout},
		{"generic error", errors.New("unknown"), ErrCodeInternal},
		{"nil", nil, ErrCodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorToCode(tt.err); got != tt.expected {
				t.Errorf("ErrorToCode(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestErrorToCode_WrappedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"wrapped ErrEntityNotFound", fmt.Errorf("getting user: %w", ErrEntityNotFound), ErrCodeNotFound},
		{"wrapped ErrValidation", fmt.Errorf("validating: %w", ErrValidation), ErrCodeValidationFailed},
		{"wrapped ErrRateLimited", fmt.Errorf("request: %w", ErrRateLimited), ErrCodeRateLimited},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorToCode(tt.err); got != tt.expected {
				t.Errorf("ErrorToCode(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestSentinelErrors_Messages(t *testing.T) {
	// Ensure sentinel errors have expected messages
	tests := []struct {
		err      error
		expected string
	}{
		{ErrEntityNotFound, "entity not found"},
		{ErrFeatureNotFound, "feature not found"},
		{ErrGroupNotFound, "feature group not found"},
		{ErrInvalidInput, "invalid input"},
		{ErrTypeMismatch, "type mismatch"},
		{ErrValidation, "validation failed"},
		{ErrStorageFull, "storage full"},
		{ErrAlreadyExists, "already exists"},
		{ErrRateLimited, "rate limited"},
		{ErrUnauthorized, "unauthorized"},
		{ErrForbidden, "forbidden"},
		{ErrTimeout, "operation timed out"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}

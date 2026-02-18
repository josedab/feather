package gitopsdefs

import "errors"

var (
	// ErrDefinitionNotFound is returned when a feature definition doesn't exist.
	ErrDefinitionNotFound = errors.New("definition not found")

	// ErrDefinitionInvalid is returned when a feature definition fails validation.
	ErrDefinitionInvalid = errors.New("definition invalid")

	// ErrReconcileFailed is returned when reconciliation fails.
	ErrReconcileFailed = errors.New("reconcile failed")
)

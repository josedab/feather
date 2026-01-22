package saascontrol

import "errors"

var (
	// ErrTenantNotFound is returned when a tenant doesn't exist.
	ErrTenantNotFound = errors.New("tenant not found")

	// ErrTenantExists is returned when a tenant already exists.
	ErrTenantExists = errors.New("tenant already exists")

	// ErrQuotaExceeded is returned when a tenant exceeds their quota.
	ErrQuotaExceeded = errors.New("quota exceeded")

	// ErrInstanceNotFound is returned when an instance doesn't exist.
	ErrInstanceNotFound = errors.New("instance not found")

	// ErrInvalidPlan is returned when a plan is not recognized.
	ErrInvalidPlan = errors.New("invalid plan")
)

package cloudcontrol

import "errors"

var (
	ErrInstanceNotFound  = errors.New("instance not found")
	ErrInstanceExists    = errors.New("instance already exists")
	ErrTenantNotFound    = errors.New("tenant not found")
	ErrTenantExists      = errors.New("tenant already exists")
	ErrQuotaExceeded     = errors.New("tenant quota exceeded")
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrInstanceNotReady  = errors.New("instance not ready")
)

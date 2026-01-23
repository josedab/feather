package auditlog

import "errors"

var (
	// ErrAuditLogFull is returned when the audit log has reached its maximum capacity.
	ErrAuditLogFull = errors.New("audit log full")

	// ErrInvalidQuery is returned when a query filter is invalid.
	ErrInvalidQuery = errors.New("invalid query")
)

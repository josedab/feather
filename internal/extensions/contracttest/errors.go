package contracttest

import "errors"

var (
	// ErrContractNotFound is returned when a contract doesn't exist.
	ErrContractNotFound = errors.New("contract not found")

	// ErrContractViolation is returned when a contract validation fails.
	ErrContractViolation = errors.New("contract violation")

	// ErrInvalidContract is returned when a contract definition is invalid.
	ErrInvalidContract = errors.New("invalid contract")
)

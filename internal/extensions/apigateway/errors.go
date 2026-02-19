package apigateway

import "errors"

var (
	// ErrBackendUnavailable is returned when no healthy backend is available to serve the request.
	ErrBackendUnavailable = errors.New("no healthy backend available")

	// ErrRateLimited is returned when a tenant has exceeded its rate limit.
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrRequestTimeout is returned when a request exceeds the configured timeout.
	ErrRequestTimeout = errors.New("request timeout")
)

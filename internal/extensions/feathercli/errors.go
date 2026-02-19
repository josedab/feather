package feathercli

import "errors"

var (
	// ErrConnectionFailed is returned when the client cannot reach the Feather server.
	ErrConnectionFailed = errors.New("connection to server failed")

	// ErrCommandFailed is returned when a CLI command fails to execute.
	ErrCommandFailed = errors.New("command execution failed")

	// ErrInvalidArgs is returned when CLI arguments are malformed or missing.
	ErrInvalidArgs = errors.New("invalid arguments")
)

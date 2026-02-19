package webhooks

import "errors"

var (
	// ErrWebhookNotFound is returned when a webhook does not exist.
	ErrWebhookNotFound = errors.New("webhook not found")

	// ErrWebhookExists is returned when a webhook with the same ID already exists.
	ErrWebhookExists = errors.New("webhook already exists")

	// ErrDeliveryFailed is returned when webhook delivery fails.
	ErrDeliveryFailed = errors.New("delivery failed")
)

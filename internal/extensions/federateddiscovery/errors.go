package federateddiscovery

import "errors"

var (
	// ErrCatalogNotFound is returned when a catalog entry doesn't exist.
	ErrCatalogNotFound = errors.New("catalog entry not found")

	// ErrFeatureRefNotFound is returned when a feature reference doesn't exist.
	ErrFeatureRefNotFound = errors.New("feature reference not found")

	// ErrSubscriptionExists is returned when a subscription already exists.
	ErrSubscriptionExists = errors.New("subscription already exists")
)

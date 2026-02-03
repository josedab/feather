// Package prefetch implements predictive feature pre-fetching using ML-based
// access pattern analysis to proactively warm features from warm→hot tier.
//
// The prefetcher instruments feature reads to learn co-access and temporal patterns,
// then uses a lightweight prediction model to anticipate future requests.
//
// # Usage
//
//	controller := prefetch.NewController(prefetch.DefaultConfig())
//	controller.RecordAccess("user:123", []string{"click_count", "purchase_total"})
//	predictions := controller.Predict("user:123")
package prefetch

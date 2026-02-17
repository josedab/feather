// Package adaptivecache provides ML-driven cache pre-warming that predicts
// feature access patterns using exponential smoothing and promotes
// predicted-hot keys from warm to hot tier.
//
// Key components:
//   - Predictor: Tracks access patterns and predicts future hot keys
//   - Prediction: Represents a predicted-hot key with score and confidence
package adaptivecache

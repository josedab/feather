// Package ml provides machine learning model integration for the feature store.
//
// It enables connecting to external ML serving platforms (TensorFlow Serving,
// Triton, SageMaker) for real-time inference using features from the store.
// The package handles feature vector construction and prediction caching.
//
// Key components:
//   - Connector: Interface for ML serving platform integration
//   - PredictionService: Orchestrates feature retrieval and model inference
package ml

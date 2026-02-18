// Package abfeatures provides multi-model A/B feature serving that routes
// different feature versions to different model variants with automatic
// traffic splitting and statistical significance testing.
//
// Key components:
//   - Manager: Manages experiments and traffic routing
//   - Experiment: Defines variants and traffic allocation
//   - SignificanceResult: Statistical test outcomes
package abfeatures

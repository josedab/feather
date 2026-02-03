// Package diffprivacy provides differential privacy mechanisms for the feature serving layer.
//
// It implements Laplace noise, Gaussian mechanism, and local DP with configurable
// per-feature privacy budgets (ε, δ), composition theorems for multi-query tracking,
// and privacy-aware aggregations with sensitivity calibration.
//
// # Usage
//
//	engine := diffprivacy.NewEngine(diffprivacy.DefaultConfig())
//	engine.RegisterFeature("click_count", diffprivacy.FeaturePrivacyConfig{
//	    Epsilon: 1.0,
//	    Delta:   1e-5,
//	    Mechanism: diffprivacy.MechanismLaplace,
//	})
//	noisyValue := engine.AddNoise("click_count", 42.0)
package diffprivacy

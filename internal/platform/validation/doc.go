// Package validation provides statistical validation for online-offline feature
// consistency checking.
//
// It extends the basic consistency checker with statistical tests, continuous
// validation pipelines, and automated reporting. The package supports multiple
// comparison methods including exact matching, numeric tolerance, statistical
// hypothesis testing, and distribution comparison.
//
// # Validation Rules
//
// Rules define how features should be compared between online and offline sources:
//
//	rule := &validation.ValidationRule{
//	    Name:          "user_clicks_check",
//	    Feature:       "user_clicks",
//	    OnlineSource:  "hot_store",
//	    OfflineSource: "data_warehouse",
//	    CompareMethod: validation.CompareNumeric,
//	    Tolerance:     0.01,
//	    SampleRate:    1.0,
//	    Enabled:       true,
//	}
//
// # Statistical Tests
//
// The package provides pure Go implementations of common statistical tests
// including Kolmogorov-Smirnov, Pearson correlation, and Population Stability
// Index for comparing feature distributions.
package validation

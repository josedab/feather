package feastcompat

import (
	"fmt"
	"time"
)

// CompatTestCase defines a compatibility test case for Feast API validation.
type CompatTestCase struct {
	Name           string
	Endpoint       string
	Method         string
	RequestBody    interface{}
	ExpectedStatus int
	Validators     []ResponseValidator
}

// ResponseValidator validates a response body from a compatibility test.
type ResponseValidator func(body []byte) error

// CompatReport summarizes the results of running the compatibility test suite.
type CompatReport struct {
	TotalTests int                `json:"total_tests"`
	Passed     int                `json:"passed"`
	Failed     int                `json:"failed"`
	Skipped    int                `json:"skipped"`
	Results    []CompatTestResult `json:"results"`
	Timestamp  time.Time          `json:"timestamp"`
}

// compatTestDef holds a test definition used by RunTest and ListTests.
type compatTestDef struct {
	name     string
	category string
	fn       func() error
}

// getTestDefs returns the full set of compatibility test definitions.
func (s *CompatTestSuite) getTestDefs() []compatTestDef {
	return []compatTestDef{
		{"get_online_features_empty", "online", s.testGetOnlineFeaturesEmpty},
		{"get_online_features_with_entity", "online", s.testGetOnlineFeaturesWithEntity},
		{"push_single_feature", "push", s.testPushSingleFeature},
		{"push_batch", "push", s.testPushBatch},
		{"apply_feature_view", "apply", s.testApplyFeatureView},
		{"apply_feature_service", "apply", s.testApplyFeatureService},
		{"list_feature_views", "apply", s.testListFeatureViews},
		{"materialize_incremental", "materialize", s.testMaterializeIncremental},
		{"saved_dataset_operations", "dataset", s.testSavedDatasetOps},
	}
}

// ListTests returns the names of all registered compatibility tests.
func (s *CompatTestSuite) ListTests() []string {
	defs := s.getTestDefs()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.name
	}
	return names
}

// RunTest runs a single named compatibility test and returns the result.
func (s *CompatTestSuite) RunTest(name string) *CompatTestResult {
	for _, td := range s.getTestDefs() {
		if td.name == name {
			start := time.Now()
			err := td.fn()
			result := &CompatTestResult{
				Name:     td.name,
				Category: td.category,
				Passed:   err == nil,
				Duration: time.Since(start),
			}
			if err != nil {
				result.Error = err.Error()
			}
			return result
		}
	}
	return &CompatTestResult{
		Name:   name,
		Passed: false,
		Error:  fmt.Sprintf("test %q not found", name),
	}
}

// Report returns a CompatReport from the most recent test results.
func (s *CompatTestSuite) Report() *CompatReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	report := &CompatReport{
		TotalTests: len(s.results),
		Results:    s.results,
		Timestamp:  s.completedAt,
	}

	for _, r := range s.results {
		if r.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}

	return report
}

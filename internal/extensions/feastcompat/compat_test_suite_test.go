package feastcompat

import (
	"testing"
)

func TestCompatTestSuiteListTests(t *testing.T) {
	gw := NewGateway(NewAdapter(DefaultAdapterConfig()))
	suite := NewCompatTestSuite(gw)

	tests := suite.ListTests()
	if len(tests) == 0 {
		t.Fatal("expected non-empty test list")
	}

	// Verify known test names exist
	expected := map[string]bool{
		"get_online_features_empty":       false,
		"get_online_features_with_entity": false,
		"push_single_feature":             false,
		"push_batch":                      false,
		"apply_feature_view":              false,
		"apply_feature_service":           false,
		"list_feature_views":              false,
		"materialize_incremental":         false,
		"saved_dataset_operations":        false,
	}
	for _, name := range tests {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected test %q not found in ListTests()", name)
		}
	}
}

func TestCompatTestSuiteRunTest(t *testing.T) {
	gw := NewGateway(NewAdapter(DefaultAdapterConfig()))
	suite := NewCompatTestSuite(gw)

	result := suite.RunTest("push_single_feature")
	if !result.Passed {
		t.Errorf("expected push_single_feature to pass, got error: %s", result.Error)
	}
	if result.Name != "push_single_feature" {
		t.Errorf("expected name push_single_feature, got %s", result.Name)
	}
	if result.Category != "push" {
		t.Errorf("expected category push, got %s", result.Category)
	}
}

func TestCompatTestSuiteRunTestNotFound(t *testing.T) {
	gw := NewGateway(NewAdapter(DefaultAdapterConfig()))
	suite := NewCompatTestSuite(gw)

	result := suite.RunTest("nonexistent_test")
	if result.Passed {
		t.Error("expected nonexistent test to fail")
	}
	if result.Error == "" {
		t.Error("expected error message for nonexistent test")
	}
}

func TestCompatTestSuiteReport(t *testing.T) {
	gw := NewGateway(NewAdapter(DefaultAdapterConfig()))
	suite := NewCompatTestSuite(gw)

	// Run all tests first
	suite.RunAll()

	report := suite.Report()
	if report.TotalTests == 0 {
		t.Fatal("expected non-zero total tests in report")
	}
	if report.Passed+report.Failed != report.TotalTests {
		t.Errorf("passed(%d) + failed(%d) != total(%d)", report.Passed, report.Failed, report.TotalTests)
	}
	if len(report.Results) != report.TotalTests {
		t.Errorf("results length(%d) != total tests(%d)", len(report.Results), report.TotalTests)
	}
}

func TestCompatTestSuiteReportEmpty(t *testing.T) {
	gw := NewGateway(NewAdapter(DefaultAdapterConfig()))
	suite := NewCompatTestSuite(gw)

	report := suite.Report()
	if report.TotalTests != 0 {
		t.Errorf("expected 0 total tests before running, got %d", report.TotalTests)
	}
}

func TestCompatTestCaseType(t *testing.T) {
	tc := CompatTestCase{
		Name:           "test_online",
		Endpoint:       "/v1/feast/online-features",
		Method:         "POST",
		ExpectedStatus: 200,
		Validators: []ResponseValidator{
			func(body []byte) error {
				return nil
			},
		},
	}
	if tc.Name != "test_online" {
		t.Errorf("expected name test_online, got %s", tc.Name)
	}
	if tc.Endpoint != "/v1/feast/online-features" {
		t.Errorf("expected endpoint /v1/feast/online-features, got %s", tc.Endpoint)
	}
	if tc.Method != "POST" {
		t.Errorf("expected method POST, got %s", tc.Method)
	}
	if tc.ExpectedStatus != 200 {
		t.Errorf("expected status 200, got %d", tc.ExpectedStatus)
	}
	if len(tc.Validators) != 1 {
		t.Errorf("expected 1 validator, got %d", len(tc.Validators))
	}
	if err := tc.Validators[0]([]byte("{}")); err != nil {
		t.Errorf("validator returned unexpected error: %v", err)
	}
}

func TestCompatReportType(t *testing.T) {
	report := CompatReport{
		TotalTests: 5,
		Passed:     4,
		Failed:     1,
		Skipped:    0,
	}
	if report.TotalTests != 5 {
		t.Errorf("expected 5 total tests, got %d", report.TotalTests)
	}
	if report.Passed != 4 {
		t.Errorf("expected 4 passed, got %d", report.Passed)
	}
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	if report.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", report.Skipped)
	}
}

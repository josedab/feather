package qualitygates

import (
	"math"
	"strings"
	"testing"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator(DefaultConfig())
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
	stats := v.Stats()
	if stats.TotalValidations != 0 {
		t.Errorf("TotalValidations = %d, want 0", stats.TotalValidations)
	}
}

func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name      string
		schema    SchemaDefinition
		wantValid bool
		wantErrs  int
	}{
		{
			name: "valid schema",
			schema: SchemaDefinition{
				Name: "user_features", EntityType: "user", Version: "1.0",
				Features: []FeatureDefinition{
					{Name: "clicks", DataType: "int64"},
					{Name: "views", DataType: "float64"},
				},
			},
			wantValid: true,
		},
		{
			name: "missing name",
			schema: SchemaDefinition{
				EntityType: "user",
				Features:   []FeatureDefinition{{Name: "f", DataType: "int64"}},
			},
			wantValid: false,
			wantErrs:  1,
		},
		{
			name: "missing entity type",
			schema: SchemaDefinition{
				Name:     "test",
				Features: []FeatureDefinition{{Name: "f", DataType: "int64"}},
			},
			wantValid: false,
			wantErrs:  1,
		},
		{
			name:      "no features",
			schema:    SchemaDefinition{Name: "test", EntityType: "user"},
			wantValid: false,
			wantErrs:  1,
		},
		{
			name: "duplicate feature names",
			schema: SchemaDefinition{
				Name: "test", EntityType: "user",
				Features: []FeatureDefinition{
					{Name: "f", DataType: "int64"},
					{Name: "f", DataType: "float64"},
				},
			},
			wantValid: false,
		},
		{
			name: "feature missing data type",
			schema: SchemaDefinition{
				Name: "test", EntityType: "user",
				Features: []FeatureDefinition{{Name: "f"}},
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(DefaultConfig())
			report, err := v.ValidateSchema(tt.schema)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v (errors: %v)", report.Valid, tt.wantValid, report.Errors)
			}
			if tt.wantErrs > 0 && len(report.Errors) < tt.wantErrs {
				t.Errorf("expected at least %d errors, got %d", tt.wantErrs, len(report.Errors))
			}
		})
	}
}

func TestAssertQuality(t *testing.T) {
	tests := []struct {
		name       string
		samples    []float64
		wantPassed bool
		wantErr    bool
	}{
		{
			name:       "good quality data",
			samples:    []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			wantPassed: true,
		},
		{
			name:       "high null rate",
			samples:    []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), 1.0},
			wantPassed: false,
		},
		{
			name:    "empty samples",
			samples: []float64{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(DefaultConfig())
			report, err := v.AssertQuality("feature", tt.samples)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v (score=%.2f, nullRate=%.2f)",
					report.Passed, tt.wantPassed, report.QualityScore, report.NullRate)
			}
			if report.QualityScore < 0 || report.QualityScore > 1.0 {
				t.Errorf("QualityScore = %f, want [0, 1]", report.QualityScore)
			}
		})
	}
}

func TestValidatePR(t *testing.T) {
	tests := []struct {
		name       string
		req        PRValidationRequest
		wantPassed bool
	}{
		{
			name: "safe addition",
			req: PRValidationRequest{
				SchemaChanges: []SchemaChange{
					{Feature: "new_feat", ChangeType: ChangeAdd},
				},
				DataSamples: map[string][]float64{
					"new_feat": {1.0, 2.0, 3.0},
				},
			},
			wantPassed: true,
		},
		{
			name: "breaking removal",
			req: PRValidationRequest{
				SchemaChanges: []SchemaChange{
					{Feature: "old_feat", ChangeType: ChangeRemove},
				},
			},
			wantPassed: false,
		},
		{
			name: "type change is breaking",
			req: PRValidationRequest{
				SchemaChanges: []SchemaChange{
					{
						Feature:    "clicks",
						ChangeType: ChangeModify,
						OldSpec:    &FeatureDefinition{Name: "clicks", DataType: "int64"},
						NewSpec:    &FeatureDefinition{Name: "clicks", DataType: "string"},
					},
				},
			},
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(DefaultConfig())
			result, err := v.ValidatePR(tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", result.Passed, tt.wantPassed)
			}
			if result.Comment == "" {
				t.Error("expected non-empty Comment")
			}
		})
	}
}

func TestEvaluateRules(t *testing.T) {
	tests := []struct {
		name        string
		blocking    bool
		result      *PRValidationResult
		wantAllowed bool
	}{
		{
			name:     "all passed",
			blocking: true,
			result: &PRValidationResult{
				Passed: true, Score: 1.0,
				Checks: []CheckResult{{Name: "c1", Status: CheckPassed}},
			},
			wantAllowed: true,
		},
		{
			name:     "failed check blocks",
			blocking: true,
			result: &PRValidationResult{
				Passed: false, Score: 0.0,
				Checks: []CheckResult{{Name: "c1", Status: CheckFailed}},
			},
			wantAllowed: false,
		},
		{
			name:     "blocking disabled",
			blocking: false,
			result: &PRValidationResult{
				Passed: false, Score: 0.0,
				Checks: []CheckResult{{Name: "c1", Status: CheckFailed}},
			},
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.EnableMergeBlocking = tt.blocking
			v := NewValidator(cfg)

			decision := v.EvaluateRules(tt.result)
			if decision.Allowed != tt.wantAllowed {
				t.Errorf("Allowed = %v, want %v (reason: %s)", decision.Allowed, tt.wantAllowed, decision.Reason)
			}
		})
	}
}

func TestGenerateReport(t *testing.T) {
	v := NewValidator(DefaultConfig())
	result := &PRValidationResult{
		Passed: true, Score: 0.9,
		Checks: []CheckResult{
			{Name: "check1", Status: CheckPassed, Message: "ok"},
			{Name: "check2", Status: CheckWarning, Message: "warn"},
		},
	}

	report := v.GenerateReport(result)
	if !strings.Contains(report, "Feature Quality Gate Report") {
		t.Error("report should contain header")
	}
	if !strings.Contains(report, "check1") {
		t.Error("report should contain check names")
	}
}

func TestValidatorStats(t *testing.T) {
	v := NewValidator(DefaultConfig())

	schema := SchemaDefinition{
		Name: "s", EntityType: "e", Version: "1",
		Features: []FeatureDefinition{{Name: "f", DataType: "int64"}},
	}
	v.ValidateSchema(schema)
	v.AssertQuality("f", []float64{1.0, 2.0, 3.0})

	stats := v.Stats()
	if stats.TotalValidations != 2 {
		t.Errorf("TotalValidations = %d, want 2", stats.TotalValidations)
	}
	if stats.SchemaValidations != 1 {
		t.Errorf("SchemaValidations = %d, want 1", stats.SchemaValidations)
	}
	if stats.QualityAssertions != 1 {
		t.Errorf("QualityAssertions = %d, want 1", stats.QualityAssertions)
	}
	if stats.AverageScore <= 0 {
		t.Error("AverageScore should be positive")
	}
}

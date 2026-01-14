package experiment

import (
	"context"
	"testing"
	"time"
)

func TestEngine_CreateExperiment(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name    string
		exp     *Experiment
		wantErr bool
	}{
		{
			name: "valid A/B test",
			exp: &Experiment{
				ID:   "exp-1",
				Name: "Test Experiment",
				Type: ExperimentTypeABTest,
				Variants: []Variant{
					{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
					{ID: "treatment", Name: "Treatment", IsControl: false, Weight: 0.5},
				},
				Allocation: AllocationConfig{
					Strategy:   AllocationDeterministic,
					Percentage: 1.0,
				},
				Metrics: []MetricConfig{
					{ID: "conversion", Name: "Conversion Rate", Type: "rate"},
				},
			},
			wantErr: false,
		},
		{
			name:    "missing ID",
			exp:     &Experiment{Name: "No ID"},
			wantErr: true,
		},
		{
			name: "missing control variant",
			exp: &Experiment{
				ID:   "exp-2",
				Name: "No Control",
				Type: ExperimentTypeABTest,
				Variants: []Variant{
					{ID: "v1", Name: "Variant 1", Weight: 0.5},
					{ID: "v2", Name: "Variant 2", Weight: 0.5},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid weights",
			exp: &Experiment{
				ID:   "exp-3",
				Name: "Bad Weights",
				Type: ExperimentTypeABTest,
				Variants: []Variant{
					{ID: "control", Name: "Control", IsControl: true, Weight: 0.3},
					{ID: "treatment", Name: "Treatment", Weight: 0.3},
				},
			},
			wantErr: true,
		},
		{
			name: "valid feature flag",
			exp: &Experiment{
				ID:        "flag-1",
				Name:      "Feature Flag",
				Type:      ExperimentTypeFeatureFlag,
				FeatureID: "new-ui",
				Variants: []Variant{
					{ID: "enabled", Name: "Enabled", Weight: 1.0, Value: true},
				},
				Allocation: AllocationConfig{
					Strategy:   AllocationDeterministic,
					Percentage: 0.1,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.CreateExperiment(tt.exp)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateExperiment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEngine_CreateExperiment_Duplicate(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "First",
		Type: ExperimentTypeFeatureFlag,
		Variants: []Variant{
			{ID: "v1", Weight: 1.0},
		},
	}

	if err := engine.CreateExperiment(exp); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	if err := engine.CreateExperiment(exp); err == nil {
		t.Error("expected error for duplicate experiment")
	}
}

func TestEngine_GetExperiment(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "Test",
		Type: ExperimentTypeFeatureFlag,
		Variants: []Variant{
			{ID: "v1", Weight: 1.0},
		},
	}
	engine.CreateExperiment(exp)

	got, err := engine.GetExperiment("exp-1")
	if err != nil {
		t.Fatalf("GetExperiment() error = %v", err)
	}

	if got.Name != "Test" {
		t.Errorf("expected name 'Test', got %s", got.Name)
	}

	_, err = engine.GetExperiment("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent experiment")
	}
}

func TestEngine_StartStopExperiment(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "Lifecycle Test",
		Type: ExperimentTypeFeatureFlag,
		Variants: []Variant{
			{ID: "v1", Weight: 1.0},
		},
	}
	engine.CreateExperiment(exp)

	// Start
	if err := engine.StartExperiment("exp-1"); err != nil {
		t.Fatalf("StartExperiment() error = %v", err)
	}

	got, _ := engine.GetExperiment("exp-1")
	if got.Status != StatusRunning {
		t.Errorf("expected status Running, got %s", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	// Pause
	if err := engine.PauseExperiment("exp-1"); err != nil {
		t.Fatalf("PauseExperiment() error = %v", err)
	}

	got, _ = engine.GetExperiment("exp-1")
	if got.Status != StatusPaused {
		t.Errorf("expected status Paused, got %s", got.Status)
	}

	// Resume
	if err := engine.StartExperiment("exp-1"); err != nil {
		t.Fatalf("StartExperiment() after pause error = %v", err)
	}

	// Stop completed
	if err := engine.StopExperiment("exp-1", true); err != nil {
		t.Fatalf("StopExperiment() error = %v", err)
	}

	got, _ = engine.GetExperiment("exp-1")
	if got.Status != StatusCompleted {
		t.Errorf("expected status Completed, got %s", got.Status)
	}
	if got.EndedAt == nil {
		t.Error("expected EndedAt to be set")
	}
}

func TestEngine_GetAssignment_NotRunning(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "Draft",
		Type: ExperimentTypeFeatureFlag,
		Variants: []Variant{
			{ID: "v1", Weight: 1.0},
		},
	}
	engine.CreateExperiment(exp)

	ctx := context.Background()
	assignment, err := engine.GetAssignment(ctx, "exp-1", "user-123", nil)
	if err != nil {
		t.Fatalf("GetAssignment() error = %v", err)
	}

	if assignment.InExperiment {
		t.Error("user should not be in experiment when status is draft")
	}
}

func TestEngine_GetAssignment_DeterministicBucketing(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "A/B Test",
		Type: ExperimentTypeABTest,
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfig{
			Strategy:   AllocationDeterministic,
			Percentage: 1.0,
			Salt:       "test-salt",
		},
	}
	engine.CreateExperiment(exp)
	engine.StartExperiment("exp-1")

	ctx := context.Background()

	// Same user should get same assignment
	assignment1, _ := engine.GetAssignment(ctx, "exp-1", "user-123", nil)
	assignment2, _ := engine.GetAssignment(ctx, "exp-1", "user-123", nil)

	if assignment1.VariantID != assignment2.VariantID {
		t.Error("deterministic bucketing should give same variant for same user")
	}

	// Different users should be distributed
	controlCount := 0
	treatmentCount := 0
	for i := 0; i < 1000; i++ {
		assignment, _ := engine.GetAssignment(ctx, "exp-1", string(rune('A'+i)), nil)
		if assignment.InExperiment {
			if assignment.VariantID == "control" {
				controlCount++
			} else {
				treatmentCount++
			}
		}
	}

	// Should be roughly 50/50 (within 20%)
	total := controlCount + treatmentCount
	if total == 0 {
		t.Fatal("no users were assigned")
	}

	ratio := float64(controlCount) / float64(total)
	if ratio < 0.3 || ratio > 0.7 {
		t.Errorf("distribution too skewed: control=%d, treatment=%d", controlCount, treatmentCount)
	}
}

func TestEngine_GetAssignment_StickyAllocation(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "Sticky Test",
		Type: ExperimentTypeABTest,
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfig{
			Strategy:   AllocationSticky,
			Percentage: 1.0,
		},
	}
	engine.CreateExperiment(exp)
	engine.StartExperiment("exp-1")

	ctx := context.Background()

	// First assignment
	assignment1, _ := engine.GetAssignment(ctx, "exp-1", "sticky-user", nil)

	// Subsequent assignments should be the same (sticky)
	for i := 0; i < 10; i++ {
		assignment2, _ := engine.GetAssignment(ctx, "exp-1", "sticky-user", nil)
		if assignment1.VariantID != assignment2.VariantID {
			t.Error("sticky allocation should return same variant")
		}
	}
}

func TestEngine_GetAssignment_TargetingRules(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "Targeted",
		Type: ExperimentTypeABTest,
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		TargetingRules: []TargetingRule{
			{ID: "r1", Attribute: "country", Operator: "eq", Value: "US"},
			{ID: "r2", Attribute: "age", Operator: "gte", Value: 18},
		},
		Allocation: AllocationConfig{
			Strategy:   AllocationDeterministic,
			Percentage: 1.0,
		},
	}
	engine.CreateExperiment(exp)
	engine.StartExperiment("exp-1")

	ctx := context.Background()

	// Matching attributes
	assignment, _ := engine.GetAssignment(ctx, "exp-1", "user-1", map[string]interface{}{
		"country": "US",
		"age":     25,
	})
	if !assignment.InExperiment {
		t.Error("user matching all targeting rules should be in experiment")
	}

	// Missing attribute
	assignment, _ = engine.GetAssignment(ctx, "exp-1", "user-2", map[string]interface{}{
		"country": "US",
	})
	if assignment.InExperiment {
		t.Error("user missing required attribute should not be in experiment")
	}

	// Non-matching attribute
	assignment, _ = engine.GetAssignment(ctx, "exp-1", "user-3", map[string]interface{}{
		"country": "UK",
		"age":     25,
	})
	if assignment.InExperiment {
		t.Error("user not matching targeting rules should not be in experiment")
	}
}

func TestEngine_GetAssignment_AllocationPercentage(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-1",
		Name: "Partial Rollout",
		Type: ExperimentTypeFeatureFlag,
		Variants: []Variant{
			{ID: "enabled", Name: "Enabled", Weight: 1.0, Value: true},
		},
		Allocation: AllocationConfig{
			Strategy:   AllocationDeterministic,
			Percentage: 0.1, // 10% of users
			Salt:       "rollout-salt",
		},
	}
	engine.CreateExperiment(exp)
	engine.StartExperiment("exp-1")

	ctx := context.Background()

	inExperiment := 0
	for i := 0; i < 1000; i++ {
		assignment, _ := engine.GetAssignment(ctx, "exp-1", string(rune(i)), nil)
		if assignment.InExperiment {
			inExperiment++
		}
	}

	// Should be roughly 10% (within 5%)
	ratio := float64(inExperiment) / 1000.0
	if ratio < 0.05 || ratio > 0.15 {
		t.Errorf("allocation percentage incorrect: got %.2f%%, expected ~10%%", ratio*100)
	}
}

func TestEngine_GetFeatureValue(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:        "flag-new-ui",
		Name:      "New UI Flag",
		Type:      ExperimentTypeFeatureFlag,
		FeatureID: "new-ui",
		Variants: []Variant{
			{ID: "enabled", Name: "Enabled", Weight: 1.0, Value: true},
		},
		Allocation: AllocationConfig{
			Strategy:   AllocationDeterministic,
			Percentage: 1.0,
		},
	}
	engine.CreateExperiment(exp)
	engine.StartExperiment("flag-new-ui")

	ctx := context.Background()

	// User in experiment gets feature value
	value, assignment, err := engine.GetFeatureValue(ctx, "new-ui", "user-1", nil, false)
	if err != nil {
		t.Fatalf("GetFeatureValue() error = %v", err)
	}

	if value != true {
		t.Errorf("expected feature value true, got %v", value)
	}

	if assignment == nil || !assignment.InExperiment {
		t.Error("expected user to be in experiment")
	}

	// Non-existent feature returns default
	value, _, _ = engine.GetFeatureValue(ctx, "nonexistent", "user-1", nil, "default")
	if value != "default" {
		t.Errorf("expected default value, got %v", value)
	}
}

func TestEngine_TrackExposure(t *testing.T) {
	engine := NewEngine()

	event := &ExposureEvent{
		ExperimentID: "exp-1",
		VariantID:    "control",
		UserID:       "user-123",
		Context:      map[string]interface{}{"page": "home"},
	}

	engine.TrackExposure(event)

	if event.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestEngine_TrackMetric(t *testing.T) {
	engine := NewEngine()

	event := &MetricEvent{
		ExperimentID: "exp-1",
		MetricID:     "conversion",
		UserID:       "user-123",
		VariantID:    "treatment",
		Value:        1.0,
	}

	engine.TrackMetric(event)

	if event.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestEngine_AnalyzeExperiment(t *testing.T) {
	engine := NewEngine()

	exp := &Experiment{
		ID:   "exp-analyze",
		Name: "Analysis Test",
		Type: ExperimentTypeABTest,
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfig{
			Strategy:   AllocationDeterministic,
			Percentage: 1.0,
		},
		Metrics: []MetricConfig{
			{ID: "conversion", Name: "Conversion Rate", Type: "rate"},
		},
	}
	engine.CreateExperiment(exp)
	engine.StartExperiment("exp-analyze")

	// Add metric events
	// Control: 10 users with values around 0.1
	for i := 0; i < 100; i++ {
		engine.TrackMetric(&MetricEvent{
			ExperimentID: "exp-analyze",
			MetricID:     "conversion",
			UserID:       string(rune('A' + i)),
			VariantID:    "control",
			Value:        0.1 + float64(i%3)*0.01,
		})
	}

	// Treatment: 10 users with values around 0.15 (50% lift)
	for i := 0; i < 100; i++ {
		engine.TrackMetric(&MetricEvent{
			ExperimentID: "exp-analyze",
			MetricID:     "conversion",
			UserID:       string(rune('A' + 100 + i)),
			VariantID:    "treatment",
			Value:        0.15 + float64(i%3)*0.01,
		})
	}

	results, err := engine.AnalyzeExperiment("exp-analyze")
	if err != nil {
		t.Fatalf("AnalyzeExperiment() error = %v", err)
	}

	if results.SampleSize != 200 {
		t.Errorf("expected sample size 200, got %d", results.SampleSize)
	}

	if len(results.VariantResults) != 2 {
		t.Fatalf("expected 2 variant results, got %d", len(results.VariantResults))
	}

	// Check treatment has positive lift
	for _, vr := range results.VariantResults {
		if vr.VariantID == "treatment" {
			for _, mr := range vr.MetricResults {
				if mr.Lift <= 0 {
					t.Errorf("expected positive lift for treatment, got %.2f", mr.Lift)
				}
			}
		}
	}

	if !results.AnalyzedAt.Before(time.Now().Add(time.Second)) {
		t.Error("expected AnalyzedAt to be set")
	}
}

func TestEngine_ListExperiments(t *testing.T) {
	engine := NewEngine()

	for i := 0; i < 5; i++ {
		exp := &Experiment{
			ID:   string(rune('a' + i)),
			Name: "Exp",
			Type: ExperimentTypeFeatureFlag,
			Variants: []Variant{
				{ID: "v1", Weight: 1.0},
			},
		}
		engine.CreateExperiment(exp)
	}

	experiments := engine.ListExperiments()
	if len(experiments) != 5 {
		t.Errorf("expected 5 experiments, got %d", len(experiments))
	}
}

func TestEngine_GetActiveExperiments(t *testing.T) {
	engine := NewEngine()

	for i := 0; i < 5; i++ {
		exp := &Experiment{
			ID:   string(rune('a' + i)),
			Name: "Exp",
			Type: ExperimentTypeFeatureFlag,
			Variants: []Variant{
				{ID: "v1", Weight: 1.0},
			},
		}
		engine.CreateExperiment(exp)
		if i < 3 {
			engine.StartExperiment(string(rune('a' + i)))
		}
	}

	active := engine.GetActiveExperiments()
	if len(active) != 3 {
		t.Errorf("expected 3 active experiments, got %d", len(active))
	}
}

func TestEngine_GetExperimentsByFeature(t *testing.T) {
	engine := NewEngine()

	exp1 := &Experiment{
		ID:        "exp-1",
		FeatureID: "feature-a",
		Type:      ExperimentTypeFeatureFlag,
		Variants:  []Variant{{ID: "v1", Weight: 1.0}},
	}
	exp2 := &Experiment{
		ID:        "exp-2",
		FeatureID: "feature-a",
		Type:      ExperimentTypeFeatureFlag,
		Variants:  []Variant{{ID: "v1", Weight: 1.0}},
	}
	exp3 := &Experiment{
		ID:        "exp-3",
		FeatureID: "feature-b",
		Type:      ExperimentTypeFeatureFlag,
		Variants:  []Variant{{ID: "v1", Weight: 1.0}},
	}

	engine.CreateExperiment(exp1)
	engine.CreateExperiment(exp2)
	engine.CreateExperiment(exp3)

	featureAExps := engine.GetExperimentsByFeature("feature-a")
	if len(featureAExps) != 2 {
		t.Errorf("expected 2 experiments for feature-a, got %d", len(featureAExps))
	}

	featureBExps := engine.GetExperimentsByFeature("feature-b")
	if len(featureBExps) != 1 {
		t.Errorf("expected 1 experiment for feature-b, got %d", len(featureBExps))
	}
}

func TestTargetingOperators(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		operator string
		value    interface{}
		attr     interface{}
		want     bool
	}{
		{"eq match", "eq", "test", "test", true},
		{"eq no match", "eq", "test", "other", false},
		{"neq match", "neq", "test", "other", true},
		{"neq no match", "neq", "test", "test", false},
		{"contains match", "contains", "est", "test", true},
		{"contains no match", "contains", "xyz", "test", false},
		{"gt match", "gt", 5, 10, true},
		{"gt no match", "gt", 10, 5, false},
		{"gte match equal", "gte", 10, 10, true},
		{"lt match", "lt", 10, 5, true},
		{"lte match equal", "lte", 10, 10, true},
		{"in match", "in", []interface{}{"a", "b", "c"}, "b", true},
		{"in no match", "in", []interface{}{"a", "b", "c"}, "d", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := &Experiment{
				ID:   "exp-" + tt.name,
				Type: ExperimentTypeFeatureFlag,
				Variants: []Variant{
					{ID: "v1", Weight: 1.0},
				},
				TargetingRules: []TargetingRule{
					{ID: "r1", Attribute: "test", Operator: tt.operator, Value: tt.value},
				},
				Allocation: AllocationConfig{
					Strategy:   AllocationDeterministic,
					Percentage: 1.0,
				},
			}
			engine.CreateExperiment(exp)
			engine.StartExperiment("exp-" + tt.name)

			ctx := context.Background()
			assignment, _ := engine.GetAssignment(ctx, "exp-"+tt.name, "user", map[string]interface{}{
				"test": tt.attr,
			})

			if assignment.InExperiment != tt.want {
				t.Errorf("expected InExperiment=%v, got %v", tt.want, assignment.InExperiment)
			}
		})
	}
}

func TestMathHelpers(t *testing.T) {
	t.Run("mean", func(t *testing.T) {
		if got := mean([]float64{1, 2, 3, 4, 5}); got != 3.0 {
			t.Errorf("mean() = %v, want 3.0", got)
		}
		if got := mean([]float64{}); got != 0 {
			t.Errorf("mean([]) = %v, want 0", got)
		}
	})

	t.Run("stddev", func(t *testing.T) {
		values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
		got := stddev(values)
		// Expected population stddev is approximately 2.138
		if got < 2.0 || got > 2.2 {
			t.Errorf("stddev() = %v, want ~2.138", got)
		}
		if got := stddev([]float64{1}); got != 0 {
			t.Errorf("stddev([1]) = %v, want 0", got)
		}
	})

	t.Run("sqrt", func(t *testing.T) {
		if got := sqrt(4); got < 1.99 || got > 2.01 {
			t.Errorf("sqrt(4) = %v, want 2.0", got)
		}
		if got := sqrt(0); got != 0 {
			t.Errorf("sqrt(0) = %v, want 0", got)
		}
	})

	t.Run("confidenceInterval", func(t *testing.T) {
		values := []float64{10, 10, 10, 10, 10}
		lower, upper := confidenceInterval(values, 0.95)
		// With no variance, CI should be tight around mean
		if lower != upper {
			t.Errorf("CI for constant values should be equal: [%v, %v]", lower, upper)
		}
	})
}

func TestAutoDecision(t *testing.T) {
	e := NewEngine()

	exp := &Experiment{
		ID:     "exp-auto",
		Name:   "Auto Decision Test",
		Type:   ExperimentTypeABTest,
		Status: StatusDraft,
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfig{Strategy: AllocationDeterministic, Percentage: 100, Salt: "salt"},
		Metrics:    []MetricConfig{{ID: "m1", Name: "conversion"}},
	}
	if err := e.CreateExperiment(exp); err != nil {
		t.Fatal(err)
	}
	if err := e.StartExperiment("exp-auto"); err != nil {
		t.Fatal(err)
	}

	config := DefaultAutoDecisionConfig()
	config.MinRunDuration = 0 // disable for test

	// Not enough samples
	result, err := e.CheckAutoDecision("exp-auto", config)
	if err != nil {
		t.Fatal(err)
	}
	if result.ShouldComplete {
		t.Error("should not complete without samples")
	}

	// Non-existent experiment
	_, err = e.CheckAutoDecision("non-existent", config)
	if err == nil {
		t.Error("expected error for non-existent experiment")
	}
}

func TestGetFeatureImpact(t *testing.T) {
	e := NewEngine()

	exp := &Experiment{
		ID:        "exp-impact",
		Name:      "Impact Test",
		Type:      ExperimentTypeABTest,
		Status:    StatusDraft,
		FeatureID: "feature-x",
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfig{Strategy: AllocationDeterministic, Percentage: 100, Salt: "salt"},
		Metrics:    []MetricConfig{{ID: "m1", Name: "conversion"}},
	}
	if err := e.CreateExperiment(exp); err != nil {
		t.Fatal(err)
	}

	impact, err := e.GetFeatureImpact("feature-x")
	if err != nil {
		t.Fatal(err)
	}
	if impact.TotalExperiments != 1 {
		t.Errorf("expected 1 experiment, got %d", impact.TotalExperiments)
	}

	// Non-existent feature
	_, err = e.GetFeatureImpact("non-existent")
	if err == nil {
		t.Error("expected error for non-existent feature")
	}
}

func TestGetExperimentSummary(t *testing.T) {
	e := NewEngine()

	exp := &Experiment{
		ID:        "exp-sum",
		Name:      "Summary Test",
		Type:      ExperimentTypeABTest,
		Status:    StatusDraft,
		FeatureID: "feat-1",
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true, Weight: 0.5},
			{ID: "treatment", Name: "Treatment", Weight: 0.5},
		},
		Allocation: AllocationConfig{Strategy: AllocationDeterministic, Percentage: 100, Salt: "s"},
		Metrics:    []MetricConfig{{ID: "m1", Name: "conv"}},
	}
	if err := e.CreateExperiment(exp); err != nil {
		t.Fatal(err)
	}

	summary := e.GetExperimentSummary()
	total, ok := summary["total_experiments"].(int)
	if !ok || total != 1 {
		t.Errorf("expected 1 total experiment, got %v", summary["total_experiments"])
	}
	ft, ok := summary["features_tested"].(int)
	if !ok || ft != 1 {
		t.Errorf("expected 1 feature tested, got %v", summary["features_tested"])
	}
}

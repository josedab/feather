package compute

import (
	"context"
	"testing"
	"time"
)

func TestDefine(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()

	def := &FeatureDefinition{
		Name:       "user_score",
		Expression: "purchase_total * 0.5 + click_count * 0.1",
		Inputs:     []string{"purchase_total", "click_count"},
		OutputType: "float64",
		Mode:       ComputeModeOnDemand,
	}

	// Define
	if err := engine.Define(ctx, def); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	// Retrieve
	got, err := engine.Get(ctx, "user_score")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "user_score" {
		t.Errorf("expected name user_score, got %s", got.Name)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// List
	defs := engine.List(ctx)
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}

	// Duplicate define should fail
	err = engine.Define(ctx, &FeatureDefinition{
		Name:       "user_score",
		Expression: "x + y",
		Inputs:     []string{"x", "y"},
		OutputType: "float64",
	})
	if err == nil {
		t.Fatal("expected error for duplicate definition")
	}

	// Undefine
	if err := engine.Undefine(ctx, "user_score"); err != nil {
		t.Fatalf("Undefine failed: %v", err)
	}
	_, err = engine.Get(ctx, "user_score")
	if err == nil {
		t.Fatal("expected error after undefine")
	}
}

func TestCompute(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()

	// Simple arithmetic
	def := &FeatureDefinition{
		Name:       "total_value",
		Expression: "price * quantity + tax",
		Inputs:     []string{"price", "quantity", "tax"},
		OutputType: "float64",
		Mode:       ComputeModeOnDemand,
	}
	if err := engine.Define(ctx, def); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	result, err := engine.Compute(ctx, "total_value", map[string]interface{}{
		"price":    10.0,
		"quantity": 3.0,
		"tax":      2.5,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	expected := 10.0*3.0 + 2.5
	if result.Value.(float64) != expected {
		t.Errorf("expected %f, got %v", expected, result.Value)
	}
	if result.CacheHit {
		t.Error("expected cache miss on first compute")
	}

	// Compute with function call
	def2 := &FeatureDefinition{
		Name:       "score",
		Expression: "max(a, b) + abs(c)",
		Inputs:     []string{"a", "b", "c"},
		OutputType: "float64",
		Mode:       ComputeModeOnDemand,
	}
	if err := engine.Define(ctx, def2); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	result, err = engine.Compute(ctx, "score", map[string]interface{}{
		"a": 5.0,
		"b": 10.0,
		"c": -3.0,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	expectedScore := 10.0 + 3.0
	if result.Value.(float64) != expectedScore {
		t.Errorf("expected %f, got %v", expectedScore, result.Value)
	}

	// Compute with missing input
	_, err = engine.Compute(ctx, "total_value", map[string]interface{}{
		"price": 10.0,
	})
	if err == nil {
		t.Fatal("expected error for missing input")
	}

	// Compute for non-existent feature
	_, err = engine.Compute(ctx, "nonexistent", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for non-existent feature")
	}
}

func TestComputeBatch(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()

	def := &FeatureDefinition{
		Name:       "doubled",
		Expression: "x * 2",
		Inputs:     []string{"x"},
		OutputType: "float64",
		Mode:       ComputeModeBatch,
	}
	if err := engine.Define(ctx, def); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	entities := []map[string]interface{}{
		{"x": 1.0},
		{"x": 2.0},
		{"x": 3.0},
	}

	results, err := engine.ComputeBatch(ctx, "doubled", entities)
	if err != nil {
		t.Fatalf("ComputeBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expected := []float64{2.0, 4.0, 6.0}
	for i, r := range results {
		if r == nil {
			t.Fatalf("result %d is nil", i)
		}
		if r.Value.(float64) != expected[i] {
			t.Errorf("result %d: expected %f, got %v", i, expected[i], r.Value)
		}
	}

	// Batch size limit
	cfg := DefaultComputeConfig()
	cfg.MaxBatchSize = 2
	engine2 := NewComputeEngine(cfg)
	defer engine2.Close()

	if err := engine2.Define(ctx, def); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	_, err = engine2.ComputeBatch(ctx, "doubled", entities)
	if err == nil {
		t.Fatal("expected error for batch size exceeded")
	}
}

func TestIncremental(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()

	def := &FeatureDefinition{
		Name:        "cached_sum",
		Expression:  "a + b",
		Inputs:      []string{"a", "b"},
		OutputType:  "float64",
		Mode:        ComputeModeOnDemand,
		Incremental: true,
		CacheTTL:    1 * time.Minute,
	}
	if err := engine.Define(ctx, def); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	inputs := map[string]interface{}{
		"a": 5.0,
		"b": 3.0,
	}

	// First compute: should be a cache miss
	result1, err := engine.Compute(ctx, "cached_sum", inputs)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}
	if result1.CacheHit {
		t.Error("expected cache miss on first compute")
	}
	if result1.Value.(float64) != 8.0 {
		t.Errorf("expected 8.0, got %v", result1.Value)
	}

	// Second compute with same inputs: should be a cache hit
	result2, err := engine.Compute(ctx, "cached_sum", inputs)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}
	if !result2.CacheHit {
		t.Error("expected cache hit on second compute with same inputs")
	}

	// Third compute with different inputs: should be a cache miss
	result3, err := engine.Compute(ctx, "cached_sum", map[string]interface{}{
		"a": 10.0,
		"b": 20.0,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}
	if result3.CacheHit {
		t.Error("expected cache miss with different inputs")
	}
	if result3.Value.(float64) != 30.0 {
		t.Errorf("expected 30.0, got %v", result3.Value)
	}

	// Check stats
	stats := engine.Stats()
	if stats.CacheHits != 1 {
		t.Errorf("expected 1 cache hit, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 2 {
		t.Errorf("expected 2 cache misses, got %d", stats.CacheMisses)
	}
}

func TestValidate(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		def     *FeatureDefinition
		wantErr bool
	}{
		{
			name:    "nil definition",
			def:     nil,
			wantErr: true,
		},
		{
			name: "empty name",
			def: &FeatureDefinition{
				Expression: "x + 1",
				OutputType: "float64",
			},
			wantErr: true,
		},
		{
			name: "empty expression",
			def: &FeatureDefinition{
				Name:       "test",
				OutputType: "float64",
			},
			wantErr: true,
		},
		{
			name: "empty output type",
			def: &FeatureDefinition{
				Name:       "test",
				Expression: "x + 1",
			},
			wantErr: true,
		},
		{
			name: "invalid mode",
			def: &FeatureDefinition{
				Name:       "test",
				Expression: "x + 1",
				OutputType: "float64",
				Mode:       "invalid_mode",
			},
			wantErr: true,
		},
		{
			name: "invalid output type",
			def: &FeatureDefinition{
				Name:       "test",
				Expression: "x + 1",
				OutputType: "complex128",
			},
			wantErr: true,
		},
		{
			name: "scheduled without schedule",
			def: &FeatureDefinition{
				Name:       "test",
				Expression: "x + 1",
				OutputType: "float64",
				Mode:       ComputeModeScheduled,
			},
			wantErr: true,
		},
		{
			name: "self-referencing input",
			def: &FeatureDefinition{
				Name:       "test",
				Expression: "test + 1",
				Inputs:     []string{"test"},
				OutputType: "float64",
			},
			wantErr: true,
		},
		{
			name: "valid definition",
			def: &FeatureDefinition{
				Name:       "valid_feature",
				Expression: "x + y",
				Inputs:     []string{"x", "y"},
				OutputType: "float64",
				Mode:       ComputeModeOnDemand,
			},
			wantErr: false,
		},
		{
			name: "valid scheduled",
			def: &FeatureDefinition{
				Name:       "scheduled_feature",
				Expression: "x + y",
				Inputs:     []string{"x", "y"},
				OutputType: "float64",
				Mode:       ComputeModeScheduled,
				Schedule:   "@every 5m",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.Validate(ctx, tt.def)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLineage(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()

	// Create a chain: base_feature -> mid_feature -> top_feature
	defs := []*FeatureDefinition{
		{
			Name:       "base_feature",
			Expression: "x + 1",
			Inputs:     []string{"x"},
			OutputType: "float64",
			Mode:       ComputeModeOnDemand,
		},
		{
			Name:       "mid_feature",
			Expression: "base_feature * 2",
			Inputs:     []string{"base_feature"},
			OutputType: "float64",
			Mode:       ComputeModeOnDemand,
		},
		{
			Name:       "top_feature",
			Expression: "mid_feature + base_feature",
			Inputs:     []string{"mid_feature", "base_feature"},
			OutputType: "float64",
			Mode:       ComputeModeOnDemand,
		},
	}

	for _, def := range defs {
		if err := engine.Define(ctx, def); err != nil {
			t.Fatalf("Define %s failed: %v", def.Name, err)
		}
	}

	// Check lineage for mid_feature
	lineage, err := engine.GetLineage(ctx, "mid_feature")
	if err != nil {
		t.Fatalf("GetLineage failed: %v", err)
	}

	if lineage.Feature != "mid_feature" {
		t.Errorf("expected feature mid_feature, got %s", lineage.Feature)
	}

	// mid_feature depends on base_feature
	if len(lineage.Dependencies) != 1 || lineage.Dependencies[0] != "base_feature" {
		t.Errorf("expected dependencies [base_feature], got %v", lineage.Dependencies)
	}

	// top_feature depends on mid_feature
	if len(lineage.Dependents) != 1 || lineage.Dependents[0] != "top_feature" {
		t.Errorf("expected dependents [top_feature], got %v", lineage.Dependents)
	}

	// Check lineage for top_feature
	topLineage, err := engine.GetLineage(ctx, "top_feature")
	if err != nil {
		t.Fatalf("GetLineage failed: %v", err)
	}
	if topLineage.Depth != 2 {
		t.Errorf("expected depth 2, got %d", topLineage.Depth)
	}
	if len(topLineage.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies for top_feature, got %d", len(topLineage.Dependencies))
	}

	// Non-existent feature
	_, err = engine.GetLineage(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent feature lineage")
	}
}

func TestScheduler(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()
	scheduler := engine.scheduler

	// Define a feature for the scheduler to use
	def := &FeatureDefinition{
		Name:       "scheduled_sum",
		Expression: "1 + 2",
		Inputs:     []string{},
		OutputType: "float64",
		Mode:       ComputeModeScheduled,
		Schedule:   "@every 1h",
	}
	if err := engine.Define(ctx, def); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	// Schedule a job
	job := &MaterializationJob{
		Name:     "test_job",
		Feature:  "scheduled_sum",
		Schedule: "@every 1h",
	}
	if err := scheduler.Schedule(ctx, job); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// List jobs
	jobs := scheduler.ListJobs(ctx)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "test_job" {
		t.Errorf("expected job name test_job, got %s", jobs[0].Name)
	}

	// Get job
	got, err := scheduler.GetJob(ctx, "test_job")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if got.Status != JobStatusIdle {
		t.Errorf("expected status idle, got %s", got.Status)
	}

	// Pause
	if err := scheduler.Pause(ctx, "test_job"); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	got, _ = scheduler.GetJob(ctx, "test_job")
	if got.Status != JobStatusPaused {
		t.Errorf("expected status paused, got %s", got.Status)
	}

	// Resume
	if err := scheduler.Resume(ctx, "test_job"); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	got, _ = scheduler.GetJob(ctx, "test_job")
	if got.Status != JobStatusIdle {
		t.Errorf("expected status idle after resume, got %s", got.Status)
	}

	// Trigger now
	if err := scheduler.TriggerNow(ctx, "test_job"); err != nil {
		t.Fatalf("TriggerNow failed: %v", err)
	}
	got, _ = scheduler.GetJob(ctx, "test_job")
	if got.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", got.RunCount)
	}

	// Duplicate schedule should fail
	err = scheduler.Schedule(ctx, &MaterializationJob{
		Name:     "test_job",
		Feature:  "scheduled_sum",
		Schedule: "@every 2h",
	})
	if err == nil {
		t.Fatal("expected error for duplicate job")
	}

	// Unschedule
	if err := scheduler.Unschedule(ctx, "test_job"); err != nil {
		t.Fatalf("Unschedule failed: %v", err)
	}
	jobs = scheduler.ListJobs(ctx)
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs after unschedule, got %d", len(jobs))
	}

	// Validation errors
	if err := scheduler.Schedule(ctx, &MaterializationJob{}); err == nil {
		t.Fatal("expected error for empty job name")
	}
	if err := scheduler.Schedule(ctx, &MaterializationJob{Name: "x"}); err == nil {
		t.Fatal("expected error for empty feature")
	}
	if err := scheduler.Schedule(ctx, &MaterializationJob{Name: "x", Feature: "y"}); err == nil {
		t.Fatal("expected error for empty schedule")
	}
}

func TestEvaluator(t *testing.T) {
	eval := NewEvaluator()

	tests := []struct {
		name     string
		expr     string
		vars     map[string]interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "simple addition",
			expr:     "a + b",
			vars:     map[string]interface{}{"a": 3.0, "b": 4.0},
			expected: 7.0,
		},
		{
			name:     "multiplication",
			expr:     "x * 2",
			vars:     map[string]interface{}{"x": 5.0},
			expected: 10.0,
		},
		{
			name:     "precedence",
			expr:     "2 + 3 * 4",
			vars:     map[string]interface{}{},
			expected: 14.0,
		},
		{
			name:     "parentheses",
			expr:     "(2 + 3) * 4",
			vars:     map[string]interface{}{},
			expected: 20.0,
		},
		{
			name:     "comparison gt",
			expr:     "x > 5",
			vars:     map[string]interface{}{"x": 10.0},
			expected: true,
		},
		{
			name:     "comparison lt",
			expr:     "x < 5",
			vars:     map[string]interface{}{"x": 10.0},
			expected: false,
		},
		{
			name:     "logical and",
			expr:     "x > 0 && y > 0",
			vars:     map[string]interface{}{"x": 1.0, "y": 2.0},
			expected: true,
		},
		{
			name:     "logical or",
			expr:     "x > 10 || y > 10",
			vars:     map[string]interface{}{"x": 1.0, "y": 20.0},
			expected: true,
		},
		{
			name:     "function abs",
			expr:     "abs(x)",
			vars:     map[string]interface{}{"x": -5.0},
			expected: 5.0,
		},
		{
			name:     "function sqrt",
			expr:     "sqrt(x)",
			vars:     map[string]interface{}{"x": 16.0},
			expected: 4.0,
		},
		{
			name:     "function pow",
			expr:     "pow(2, 10)",
			vars:     map[string]interface{}{},
			expected: 1024.0,
		},
		{
			name:     "if_then_else true",
			expr:     `if_then_else(x > 0, x, 0)`,
			vars:     map[string]interface{}{"x": 5.0},
			expected: 5.0,
		},
		{
			name:     "if_then_else false",
			expr:     `if_then_else(x > 0, x, 0)`,
			vars:     map[string]interface{}{"x": -5.0},
			expected: float64(0),
		},
		{
			name:     "negation",
			expr:     "-x",
			vars:     map[string]interface{}{"x": 5.0},
			expected: -5.0,
		},
		{
			name:     "division",
			expr:     "x / y",
			vars:     map[string]interface{}{"x": 10.0, "y": 4.0},
			expected: 2.5,
		},
		{
			name:    "division by zero",
			expr:    "x / 0",
			vars:    map[string]interface{}{"x": 10.0},
			wantErr: true,
		},
		{
			name:    "undefined variable",
			expr:    "z + 1",
			vars:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name:     "boolean literal",
			expr:     "true",
			vars:     map[string]interface{}{},
			expected: true,
		},
		{
			name:     "coalesce",
			expr:     "coalesce(x, 42)",
			vars:     map[string]interface{}{"x": nil},
			expected: float64(42),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(tt.expr, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Evaluate() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("Evaluate() = %v (%T), expected %v (%T)", result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestStats(t *testing.T) {
	engine := NewComputeEngine(DefaultComputeConfig())
	defer engine.Close()

	ctx := context.Background()

	def := &FeatureDefinition{
		Name:       "stat_feature",
		Expression: "x + 1",
		Inputs:     []string{"x"},
		OutputType: "float64",
		Mode:       ComputeModeOnDemand,
	}
	if err := engine.Define(ctx, def); err != nil {
		t.Fatalf("Define failed: %v", err)
	}

	// Compute once
	_, _ = engine.Compute(ctx, "stat_feature", map[string]interface{}{"x": 1.0})

	stats := engine.Stats()
	if stats.DefinitionCount != 1 {
		t.Errorf("expected 1 definition, got %d", stats.DefinitionCount)
	}
	if stats.ComputeCount != 1 {
		t.Errorf("expected 1 compute, got %d", stats.ComputeCount)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}

	// Trigger an error
	_, _ = engine.Compute(ctx, "nonexistent", map[string]interface{}{})
	stats = engine.Stats()
	if stats.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", stats.ErrorCount)
	}
}

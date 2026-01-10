package feather

import (
	"context"
	"testing"
	"time"
)

func TestPipelineBuilder(t *testing.T) {
	pipeline := NewPipelineBuilder("user-features").
		Description("Compute user behavior features").
		Owner("ml-team").
		Tags("user", "behavior", "ml").
		Aggregate("sum_clicks", "click_events", "total_clicks", "sum", nil).
		Transform("log_clicks", "total_clicks", "log_clicks", "log(total_clicks + 1)").
		Normalize("norm_clicks", "log_clicks", "norm_log_clicks", "z_score").
		Build()

	if pipeline.Name != "user-features" {
		t.Errorf("expected name user-features, got %s", pipeline.Name)
	}

	if len(pipeline.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(pipeline.Steps))
	}

	if pipeline.Steps[0].Type != StepTypeAggregation {
		t.Errorf("expected first step to be aggregation, got %s", pipeline.Steps[0].Type)
	}

	if pipeline.Steps[1].Type != StepTypeTransform {
		t.Errorf("expected second step to be transform, got %s", pipeline.Steps[1].Type)
	}

	if pipeline.Steps[2].Type != StepTypeNormalize {
		t.Errorf("expected third step to be normalize, got %s", pipeline.Steps[2].Type)
	}
}

func TestPipelineBuilderAllSteps(t *testing.T) {
	pipeline := NewPipelineBuilder("test-all-steps").
		Aggregate("agg", "input", "agg_out", "sum", nil).
		Transform("trans", "input", "trans_out", "expr").
		Join("join", []string{"a", "b"}, "join_out", "inner", "id").
		Filter("filter", "input", "filter_out", "value > 0").
		Window("window", "input", "window_out", time.Hour, 10*time.Minute).
		Lookup("lookup", "input", "other", "lookup_out", "key").
		Normalize("norm", "input", "norm_out", "z_score").
		Bucketize("bucket", "input", "bucket_out", []float64{0, 10, 100}).
		OneHotEncode("onehot", "input", "onehot_out", []string{"a", "b", "c"}).
		Embedding("embed", "input", "embed_out", "text-embedding", 768).
		Expression("expr", []string{"a", "b"}, "expr_out", "a + b").
		Custom("custom", []string{"x"}, "custom_out", map[string]interface{}{"param": 1}).
		Build()

	if len(pipeline.Steps) != 12 {
		t.Errorf("expected 12 steps, got %d", len(pipeline.Steps))
	}

	// Verify step types
	expectedTypes := []ComputeStepType{
		StepTypeAggregation,
		StepTypeTransform,
		StepTypeJoin,
		StepTypeFilter,
		StepTypeWindow,
		StepTypeLookup,
		StepTypeNormalize,
		StepTypeBucketize,
		StepTypeOneHotEncode,
		StepTypeEmbedding,
		StepTypeExpression,
		StepTypeCustom,
	}

	for i, expected := range expectedTypes {
		if pipeline.Steps[i].Type != expected {
			t.Errorf("step %d: expected type %s, got %s", i, expected, pipeline.Steps[i].Type)
		}
	}
}

func TestLocalCompute(t *testing.T) {
	lc := NewLocalCompute()
	ctx := context.Background()

	// Test aggregation - sum
	t.Run("aggregation_sum", func(t *testing.T) {
		result, err := lc.Compute(ctx, "aggregation", map[string]interface{}{
			"values":               []float64{1, 2, 3, 4, 5},
			"_config_aggregation": "sum",
		})
		if err != nil {
			t.Fatalf("compute failed: %v", err)
		}
		if result != 15.0 {
			t.Errorf("expected 15, got %v", result)
		}
	})

	// Test aggregation - avg
	t.Run("aggregation_avg", func(t *testing.T) {
		result, err := lc.Compute(ctx, "aggregation", map[string]interface{}{
			"values":               []float64{2, 4, 6, 8, 10},
			"_config_aggregation": "avg",
		})
		if err != nil {
			t.Fatalf("compute failed: %v", err)
		}
		if result != 6.0 {
			t.Errorf("expected 6, got %v", result)
		}
	})

	// Test aggregation - count
	t.Run("aggregation_count", func(t *testing.T) {
		result, err := lc.Compute(ctx, "aggregation", map[string]interface{}{
			"values":               []float64{1, 2, 3},
			"_config_aggregation": "count",
		})
		if err != nil {
			t.Fatalf("compute failed: %v", err)
		}
		if result != 3 {
			t.Errorf("expected 3, got %v", result)
		}
	})

	// Test aggregation - min
	t.Run("aggregation_min", func(t *testing.T) {
		result, err := lc.Compute(ctx, "aggregation", map[string]interface{}{
			"values":               []float64{5, 2, 8, 1, 9},
			"_config_aggregation": "min",
		})
		if err != nil {
			t.Fatalf("compute failed: %v", err)
		}
		if result != 1.0 {
			t.Errorf("expected 1, got %v", result)
		}
	})

	// Test aggregation - max
	t.Run("aggregation_max", func(t *testing.T) {
		result, err := lc.Compute(ctx, "aggregation", map[string]interface{}{
			"values":               []float64{5, 2, 8, 1, 9},
			"_config_aggregation": "max",
		})
		if err != nil {
			t.Fatalf("compute failed: %v", err)
		}
		if result != 9.0 {
			t.Errorf("expected 9, got %v", result)
		}
	})
}

func TestLocalComputeBucketize(t *testing.T) {
	lc := NewLocalCompute()
	ctx := context.Background()

	tests := []struct {
		name       string
		value      float64
		boundaries []interface{}
		expected   int
	}{
		{"below_first", 5, []interface{}{10.0, 100.0}, 0},
		{"between", 50, []interface{}{10.0, 100.0}, 1},
		{"above_last", 150, []interface{}{10.0, 100.0}, 2},
		{"at_boundary", 10, []interface{}{10.0, 100.0}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lc.Compute(ctx, "bucketize", map[string]interface{}{
				"input":              tt.value,
				"_config_boundaries": tt.boundaries,
			})
			if err != nil {
				t.Fatalf("compute failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected bucket %d, got %v", tt.expected, result)
			}
		})
	}
}

func TestLocalComputeOneHotEncode(t *testing.T) {
	lc := NewLocalCompute()
	ctx := context.Background()

	result, err := lc.Compute(ctx, "one_hot_encode", map[string]interface{}{
		"input":              "b",
		"_config_categories": []interface{}{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}

	encoded, ok := result.([]int)
	if !ok {
		t.Fatalf("expected []int, got %T", result)
	}

	expected := []int{0, 1, 0}
	for i, v := range expected {
		if encoded[i] != v {
			t.Errorf("position %d: expected %d, got %d", i, v, encoded[i])
		}
	}
}

func TestLocalComputeCustomFunction(t *testing.T) {
	lc := NewLocalCompute()
	ctx := context.Background()

	// Register custom function
	lc.Register("double", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		val, _ := inputs["value"].(float64)
		return val * 2, nil
	})

	result, err := lc.Compute(ctx, "double", map[string]interface{}{
		"value": 21.0,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}
	if result != 42.0 {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestLocalComputeExecutePipeline(t *testing.T) {
	lc := NewLocalCompute()
	ctx := context.Background()

	pipeline := NewPipelineBuilder("test-pipeline").
		Aggregate("sum_values", "values", "total", "sum", nil).
		Build()

	results, err := lc.ExecutePipeline(ctx, pipeline, map[string]interface{}{
		"values": []float64{1, 2, 3, 4, 5},
	})
	if err != nil {
		t.Fatalf("execute pipeline failed: %v", err)
	}

	total, ok := results["total"]
	if !ok {
		t.Fatal("expected 'total' in results")
	}

	if total != 15.0 {
		t.Errorf("expected 15, got %v", total)
	}
}

func TestLocalComputeUnknownFunction(t *testing.T) {
	lc := NewLocalCompute()
	ctx := context.Background()

	_, err := lc.Compute(ctx, "nonexistent", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

func TestLocalComputeUnknownAggregation(t *testing.T) {
	lc := NewLocalCompute()
	ctx := context.Background()

	_, err := lc.Compute(ctx, "aggregation", map[string]interface{}{
		"values":               []float64{1, 2, 3},
		"_config_aggregation": "unknown_agg",
	})
	if err == nil {
		t.Error("expected error for unknown aggregation type")
	}
}

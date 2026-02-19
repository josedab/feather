package transform

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

func createTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   100 * 1024 * 1024,
		WarmInMemory: true,
	}, nil)
	require.NoError(t, err)
	return store
}

func TestNewPipeline(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	assert.NotNil(t, pipeline)
	assert.NotNil(t, pipeline.executors[TypeArithmetic])
	assert.NotNil(t, pipeline.executors[TypeAggregation])
	assert.NotNil(t, pipeline.executors[TypeConditional])
}

func TestPipeline_ArithmeticTransform(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	// Add test features
	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"price":    {Value: 100.0, Timestamp: time.Now().UnixNano()},
		"quantity": {Value: 5.0, Timestamp: time.Now().UnixNano()},
		"discount": {Value: 0.1, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	// Register transform
	transform := &Transform{
		Name:       "total_price",
		Type:       TypeArithmetic,
		Expression: "price * quantity * (1 - discount)",
		Inputs:     []string{"price", "quantity", "discount"},
		Output:     "total_price",
	}

	err = pipeline.RegisterTransform(transform)
	require.NoError(t, err)

	// Execute
	ctx := context.Background()
	result, err := pipeline.Execute(ctx, "total_price", "user:1")
	require.NoError(t, err)

	// 100 * 5 * 0.9 = 450
	assert.InDelta(t, 450.0, result.(float64), 0.01)
}

func TestPipeline_AggregationTransform(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	// Add test features
	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"score_1": {Value: 10.0, Timestamp: time.Now().UnixNano()},
		"score_2": {Value: 20.0, Timestamp: time.Now().UnixNano()},
		"score_3": {Value: 30.0, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	// Sum transform
	sumTransform := &Transform{
		Name:   "total_score",
		Type:   TypeAggregation,
		Inputs: []string{"score_1", "score_2", "score_3"},
		Output: "total_score",
		Config: map[string]interface{}{"type": "sum"},
	}

	err = pipeline.RegisterTransform(sumTransform)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := pipeline.Execute(ctx, "total_score", "user:1")
	require.NoError(t, err)
	assert.Equal(t, 60.0, result)

	// Avg transform
	avgTransform := &Transform{
		Name:   "avg_score",
		Type:   TypeAggregation,
		Inputs: []string{"score_1", "score_2", "score_3"},
		Output: "avg_score",
		Config: map[string]interface{}{"type": "avg"},
	}

	err = pipeline.RegisterTransform(avgTransform)
	require.NoError(t, err)

	result, err = pipeline.Execute(ctx, "avg_score", "user:1")
	require.NoError(t, err)
	assert.Equal(t, 20.0, result)
}

func TestPipeline_ConditionalTransform(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	// Add test features
	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"age": {Value: 25.0, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	err = store.Put(context.Background(), "user:2", map[string]*domain.FeatureValue{
		"age": {Value: 15.0, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	// Register conditional transform
	transform := &Transform{
		Name:       "age_group",
		Type:       TypeConditional,
		Expression: "age >= 18 ? 'adult' : 'minor'",
		Inputs:     []string{"age"},
		Output:     "age_group",
	}

	err = pipeline.RegisterTransform(transform)
	require.NoError(t, err)

	ctx := context.Background()

	// User 1 is adult
	result, err := pipeline.Execute(ctx, "age_group", "user:1")
	require.NoError(t, err)
	assert.Equal(t, "adult", result)

	// User 2 is minor
	result, err = pipeline.Execute(ctx, "age_group", "user:2")
	require.NoError(t, err)
	assert.Equal(t, "minor", result)
}

func TestPipeline_StringTransform(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"name": {Value: "  John Doe  ", Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		operation string
		expected  interface{}
	}{
		{"trim_name", "trim", "John Doe"},
		{"lower_name", "lower", "  john doe  "},
		{"upper_name", "upper", "  JOHN DOE  "},
		{"name_length", "length", float64(12)},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &Transform{
				Name:   tt.name,
				Type:   TypeString,
				Inputs: []string{"name"},
				Output: tt.name,
				Config: map[string]interface{}{"operation": tt.operation},
			}

			err := pipeline.RegisterTransform(transform)
			require.NoError(t, err)

			result, err := pipeline.Execute(ctx, tt.name, "user:1")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPipeline_TimestampTransform(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	testTime := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)

	err := store.Put(context.Background(), "event:1", map[string]*domain.FeatureValue{
		"created_at": {Value: testTime.UnixNano(), Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	transform := &Transform{
		Name:   "created_month",
		Type:   TypeTimestamp,
		Inputs: []string{"created_at"},
		Output: "created_month",
		Config: map[string]interface{}{"operation": "month"},
	}

	err = pipeline.RegisterTransform(transform)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := pipeline.Execute(ctx, "created_month", "event:1")
	require.NoError(t, err)
	assert.Equal(t, float64(6), result)
}

func TestPipeline_ExecuteAndStore(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"a": {Value: 10.0, Timestamp: time.Now().UnixNano()},
		"b": {Value: 20.0, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	transform := &Transform{
		Name:       "sum_ab",
		Type:       TypeArithmetic,
		Expression: "a + b",
		Inputs:     []string{"a", "b"},
		Output:     "sum_ab",
	}

	err = pipeline.RegisterTransform(transform)
	require.NoError(t, err)

	ctx := context.Background()
	err = pipeline.ExecuteAndStore(ctx, "sum_ab", "user:1")
	require.NoError(t, err)

	// Verify stored
	values, err := store.Get(context.Background(), "user:1", []string{"sum_ab"})
	require.NoError(t, err)
	assert.Equal(t, 30.0, values["sum_ab"].Value)
}

func TestPipeline_DependencyChain(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	// Raw features
	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"base_price": {Value: 100.0, Timestamp: time.Now().UnixNano()},
		"tax_rate":   {Value: 0.1, Timestamp: time.Now().UnixNano()},
		"discount":   {Value: 0.2, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	// First derived feature: price after discount
	t1 := &Transform{
		Name:       "discounted_price",
		Type:       TypeArithmetic,
		Expression: "base_price * (1 - discount)",
		Inputs:     []string{"base_price", "discount"},
		Output:     "discounted_price",
	}
	require.NoError(t, pipeline.RegisterTransform(t1))

	// Second derived feature: final price with tax (depends on discounted_price)
	t2 := &Transform{
		Name:       "final_price",
		Type:       TypeArithmetic,
		Expression: "discounted_price * (1 + tax_rate)",
		Inputs:     []string{"discounted_price", "tax_rate"},
		Output:     "final_price",
	}
	require.NoError(t, pipeline.RegisterTransform(t2))

	ctx := context.Background()

	// Execute chain
	result, err := pipeline.ExecuteChain(ctx, "final_price", "user:1")
	require.NoError(t, err)

	// 100 * 0.8 * 1.1 = 88
	assert.InDelta(t, 88.0, result.(float64), 0.01)
}

func TestPipeline_CycleDetection(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	// Create transforms that would form a cycle
	t1 := &Transform{
		Name:       "a_to_b",
		Type:       TypeArithmetic,
		Expression: "a + 1",
		Inputs:     []string{"a"},
		Output:     "b",
	}
	require.NoError(t, pipeline.RegisterTransform(t1))

	t2 := &Transform{
		Name:       "b_to_c",
		Type:       TypeArithmetic,
		Expression: "b + 1",
		Inputs:     []string{"b"},
		Output:     "c",
	}
	require.NoError(t, pipeline.RegisterTransform(t2))

	// This would create a cycle: c -> a -> b -> c
	t3 := &Transform{
		Name:       "c_to_a",
		Type:       TypeArithmetic,
		Expression: "c + 1",
		Inputs:     []string{"c"},
		Output:     "a",
	}
	err := pipeline.RegisterTransform(t3)
	assert.ErrorIs(t, err, ErrDependencyCycle)
}

func TestPipeline_ListTransforms(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	// Register multiple transforms
	for i := 0; i < 3; i++ {
		transform := &Transform{
			Name:       "transform_" + string(rune('a'+i)),
			Type:       TypeArithmetic,
			Expression: "x + 1",
			Inputs:     []string{"x"},
			Output:     "y_" + string(rune('a'+i)),
		}
		require.NoError(t, pipeline.RegisterTransform(transform))
	}

	transforms := pipeline.ListTransforms()
	assert.Len(t, transforms, 3)
}

func TestPipeline_UnregisterTransform(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)

	transform := &Transform{
		Name:       "test",
		Type:       TypeArithmetic,
		Expression: "x + 1",
		Inputs:     []string{"x"},
		Output:     "y",
	}
	require.NoError(t, pipeline.RegisterTransform(transform))

	// Unregister
	err := pipeline.UnregisterTransform("test")
	require.NoError(t, err)

	// Verify removed
	_, err = pipeline.GetTransform("test")
	assert.ErrorIs(t, err, ErrTransformNotFound)

	// Unregister non-existent
	err = pipeline.UnregisterTransform("nonexistent")
	assert.ErrorIs(t, err, ErrTransformNotFound)
}

func TestDSL_Define(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)
	dsl := NewDSL(pipeline)

	// Add test features
	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"x": {Value: 10.0, Timestamp: time.Now().UnixNano()},
		"y": {Value: 20.0, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	// Define arithmetic transform
	err = dsl.Define("add_xy", "z = x + y")
	require.NoError(t, err)

	ctx := context.Background()
	result, err := pipeline.Execute(ctx, "add_xy", "user:1")
	require.NoError(t, err)
	assert.Equal(t, 30.0, result)
}

func TestDSL_FunctionCalls(t *testing.T) {
	store := createTestStore(t)
	pipeline := NewPipeline(store)
	dsl := NewDSL(pipeline)

	// Add test features
	err := store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"a": {Value: 10.0, Timestamp: time.Now().UnixNano()},
		"b": {Value: 20.0, Timestamp: time.Now().UnixNano()},
		"c": {Value: 30.0, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	// Define aggregation transform
	err = dsl.Define("sum_all", "total = sum(a, b, c)")
	require.NoError(t, err)

	ctx := context.Background()
	result, err := pipeline.Execute(ctx, "sum_all", "user:1")
	require.NoError(t, err)
	assert.Equal(t, 60.0, result)
}

func TestParseAndEvaluate(t *testing.T) {
	tests := []struct {
		expr     string
		expected float64
	}{
		{"1 + 2", 3},
		{"5 - 3", 2},
		{"4 * 3", 12},
		{"10 / 2", 5},
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
		{"10 - 2 - 3", 5},
		{"100 / 10 / 2", 5},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := parseAndEvaluate(tt.expr)
			require.NoError(t, err)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestEvaluateCondition(t *testing.T) {
	inputs := map[string]interface{}{
		"x": 10.0,
		"y": 20.0,
		"s": "hello",
	}

	tests := []struct {
		condition string
		expected  bool
	}{
		{"x == 10", true},
		{"x != 10", false},
		{"x > 5", true},
		{"x < 5", false},
		{"x >= 10", true},
		{"x <= 10", true},
		{"y > x", true},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			result, err := evaluateCondition(tt.condition, inputs)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

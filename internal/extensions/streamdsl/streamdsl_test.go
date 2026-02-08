package streamdsl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSpec() PipelineSpec {
	return PipelineSpec{
		Name:    "test-pipeline",
		Version: "1.0",
		Sources: []SourceSpec{
			{Name: "clicks", Type: "kafka", Config: map[string]string{"topic": "clicks"}},
		},
		Transforms: []TransformSpec{
			{Name: "enriched", Input: "clicks", Expression: "value * 2"},
		},
		Sinks: []SinkSpec{
			{Name: "output", Input: "enriched", Type: "feature_store"},
		},
	}
}

func TestCompiler_Compile(t *testing.T) {
	tests := []struct {
		name    string
		spec    PipelineSpec
		wantErr bool
		check   func(t *testing.T, plan *ExecutionPlan)
	}{
		{
			name: "valid simple pipeline",
			spec: validSpec(),
			check: func(t *testing.T, plan *ExecutionPlan) {
				assert.Equal(t, "test-pipeline", plan.Name)
				assert.Equal(t, "compiled", plan.Status)
				assert.Len(t, plan.Nodes, 3)
				assert.Len(t, plan.Edges, 2)
				// Topological order: source before transform before sink.
				assert.Equal(t, NodeSource, plan.Nodes[0].Type)
				assert.Equal(t, NodeTransform, plan.Nodes[1].Type)
				assert.Equal(t, NodeSink, plan.Nodes[2].Type)
			},
		},
		{
			name: "pipeline with window and aggregation",
			spec: PipelineSpec{
				Name:    "windowed",
				Version: "1.0",
				Sources: []SourceSpec{{Name: "events", Type: "kafka"}},
				Windows: []WindowSpec{
					{Name: "win", Input: "events", Type: WindowTumbling, Size: 5 * time.Minute},
				},
				Aggregations: []AggregationSpec{
					{Name: "count_events", Input: "win", Function: AggCount, Field: "id", GroupBy: []string{"user_id"}},
				},
				Sinks: []SinkSpec{{Name: "out", Input: "count_events", Type: "feature_store"}},
			},
			check: func(t *testing.T, plan *ExecutionPlan) {
				assert.Len(t, plan.Nodes, 4)
				assert.Equal(t, NodeSource, plan.Nodes[0].Type)
				assert.Equal(t, NodeWindow, plan.Nodes[1].Type)
				assert.Equal(t, NodeAggregate, plan.Nodes[2].Type)
				assert.Equal(t, NodeSink, plan.Nodes[3].Type)
			},
		},
		{
			name: "pipeline with join",
			spec: PipelineSpec{
				Name:    "joined",
				Version: "1.0",
				Sources: []SourceSpec{
					{Name: "left_src", Type: "kafka"},
					{Name: "right_src", Type: "http"},
				},
				Joins: []JoinSpec{
					{Name: "merged", Left: "left_src", Right: "right_src", On: "user_id", Type: "inner"},
				},
				Sinks: []SinkSpec{{Name: "sink", Input: "merged", Type: "stdout"}},
			},
			check: func(t *testing.T, plan *ExecutionPlan) {
				assert.Len(t, plan.Nodes, 4)
				assert.Len(t, plan.Edges, 3)
			},
		},
		{
			name: "pipeline with filter",
			spec: PipelineSpec{
				Name:    "filtered",
				Version: "1.0",
				Sources: []SourceSpec{{Name: "raw", Type: "kafka"}},
				Filters: []FilterSpec{{Name: "active_only", Input: "raw", Condition: "status == 'active'"}},
				Sinks:   []SinkSpec{{Name: "sink", Input: "active_only", Type: "feature_store"}},
			},
			check: func(t *testing.T, plan *ExecutionPlan) {
				assert.Len(t, plan.Nodes, 3)
				assert.Equal(t, NodeFilter, plan.Nodes[1].Type)
			},
		},
	}

	compiler := NewCompiler(DefaultCompilerConfig())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := compiler.Compile(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, plan)
			}
		})
	}
}

func TestCompiler_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		spec        PipelineSpec
		errContains string
	}{
		{
			name:        "empty name",
			spec:        PipelineSpec{Sources: []SourceSpec{{Name: "s", Type: "kafka"}}, Sinks: []SinkSpec{{Name: "k", Input: "s", Type: "stdout"}}},
			errContains: "pipeline name is required",
		},
		{
			name:        "no sources",
			spec:        PipelineSpec{Name: "p", Sinks: []SinkSpec{{Name: "k", Input: "x", Type: "stdout"}}},
			errContains: "at least one source is required",
		},
		{
			name:        "no sinks",
			spec:        PipelineSpec{Name: "p", Sources: []SourceSpec{{Name: "s", Type: "kafka"}}},
			errContains: "at least one sink is required",
		},
		{
			name: "unknown input reference",
			spec: PipelineSpec{
				Name:    "p",
				Sources: []SourceSpec{{Name: "s", Type: "kafka"}},
				Sinks:   []SinkSpec{{Name: "k", Input: "nonexistent", Type: "stdout"}},
			},
			errContains: "references unknown input",
		},
		{
			name: "duplicate node name",
			spec: PipelineSpec{
				Name:    "p",
				Sources: []SourceSpec{{Name: "dup", Type: "kafka"}, {Name: "dup", Type: "http"}},
				Sinks:   []SinkSpec{{Name: "k", Input: "dup", Type: "stdout"}},
			},
			errContains: "duplicate node name",
		},
		{
			name: "window size zero",
			spec: PipelineSpec{
				Name:    "p",
				Sources: []SourceSpec{{Name: "s", Type: "kafka"}},
				Windows: []WindowSpec{{Name: "w", Input: "s", Type: WindowTumbling, Size: 0}},
				Sinks:   []SinkSpec{{Name: "k", Input: "w", Type: "stdout"}},
			},
			errContains: "size must be > 0",
		},
		{
			name: "invalid join type",
			spec: PipelineSpec{
				Name:    "p",
				Sources: []SourceSpec{{Name: "a", Type: "kafka"}, {Name: "b", Type: "kafka"}},
				Joins:   []JoinSpec{{Name: "j", Left: "a", Right: "b", On: "id", Type: "cross"}},
				Sinks:   []SinkSpec{{Name: "k", Input: "j", Type: "stdout"}},
			},
			errContains: "invalid type",
		},
		{
			name: "invalid aggregation function",
			spec: PipelineSpec{
				Name:         "p",
				Sources:      []SourceSpec{{Name: "s", Type: "kafka"}},
				Aggregations: []AggregationSpec{{Name: "a", Input: "s", Function: "median", Field: "x"}},
				Sinks:        []SinkSpec{{Name: "k", Input: "a", Type: "stdout"}},
			},
			errContains: "invalid function",
		},
		{
			name: "disallowed source type",
			spec: PipelineSpec{
				Name:    "p",
				Sources: []SourceSpec{{Name: "s", Type: "redis"}},
				Sinks:   []SinkSpec{{Name: "k", Input: "s", Type: "stdout"}},
			},
			errContains: "not allowed",
		},
		{
			name: "disallowed sink type",
			spec: PipelineSpec{
				Name:    "p",
				Sources: []SourceSpec{{Name: "s", Type: "kafka"}},
				Sinks:   []SinkSpec{{Name: "k", Input: "s", Type: "s3"}},
			},
			errContains: "not allowed",
		},
	}

	compiler := NewCompiler(DefaultCompilerConfig())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compiler.Compile(tt.spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestCompiler_CycleDetection(t *testing.T) {
	// Construct a cycle: A -> B -> C -> A by using transforms that reference each other.
	// We need sources so validation passes, then engineer a cycle through transforms.
	spec := PipelineSpec{
		Name:    "cyclic",
		Version: "1.0",
		Sources: []SourceSpec{{Name: "src", Type: "kafka"}},
		Transforms: []TransformSpec{
			{Name: "a", Input: "src", Expression: "x"},
			{Name: "b", Input: "a", Expression: "x"},
			{Name: "c", Input: "b", Expression: "x"},
		},
		// Sink points to "a", but "a" depends on "src" — no cycle here.
		// To force a cycle we'd need "a" to depend on "c", but spec validation
		// prevents a transform from referencing a later node because we build
		// edges from input. Instead we directly test topoSort.
		Sinks: []SinkSpec{{Name: "out", Input: "c", Type: "stdout"}},
	}

	// The above spec is actually a valid DAG. To test cycle detection we call topoSort directly.
	nodes := []ExecutionNode{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	}
	edges := []ExecutionEdge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "c", To: "a"},
	}

	_, err := topoSort(nodes, edges)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")

	// Verify the valid spec compiles without cycle error.
	compiler := NewCompiler(DefaultCompilerConfig())
	plan, err := compiler.Compile(spec)
	require.NoError(t, err)
	assert.Equal(t, "compiled", plan.Status)
}

func TestPipelineManager_ListAndDelete(t *testing.T) {
	pm := NewPipelineManager(DefaultCompilerConfig())

	plan1, err := pm.Compile(validSpec())
	require.NoError(t, err)

	spec2 := validSpec()
	spec2.Name = "second-pipeline"
	plan2, err := pm.Compile(spec2)
	require.NoError(t, err)

	// List should return both.
	list := pm.List()
	assert.Len(t, list, 2)

	// Get by ID.
	got, err := pm.Get(plan1.ID)
	require.NoError(t, err)
	assert.Equal(t, plan1.ID, got.ID)

	// Delete one.
	err = pm.Delete(plan2.ID)
	require.NoError(t, err)
	assert.Len(t, pm.List(), 1)

	// Delete non-existent.
	err = pm.Delete("no-such-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Get non-existent.
	_, err = pm.Get("no-such-id")
	require.Error(t, err)
}

func TestPipelineManager_Validate(t *testing.T) {
	pm := NewPipelineManager(DefaultCompilerConfig())

	errs := pm.Validate(PipelineSpec{})
	assert.NotEmpty(t, errs)

	errs = pm.Validate(validSpec())
	assert.Empty(t, errs)
}

func TestPipelineManager_Stats(t *testing.T) {
	pm := NewPipelineManager(DefaultCompilerConfig())

	stats := pm.Stats()
	assert.Equal(t, 0, stats.TotalPipelines)
	assert.Equal(t, 0.0, stats.AvgNodesPerPlan)

	_, err := pm.Compile(validSpec())
	require.NoError(t, err)

	spec2 := validSpec()
	spec2.Name = "pipeline-2"
	_, err = pm.Compile(spec2)
	require.NoError(t, err)

	stats = pm.Stats()
	assert.Equal(t, 2, stats.TotalPipelines)
	assert.Equal(t, 6, stats.TotalNodes) // 3 nodes each
	assert.Equal(t, 3.0, stats.AvgNodesPerPlan)
	assert.Equal(t, 2, stats.ByStatus["compiled"])
}

package materialization

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestEngine_RegisterPipeline(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	tests := []struct {
		name    string
		pipe    *Pipeline
		wantErr error
	}{
		{
			name: "valid pipeline",
			pipe: &Pipeline{
				Name:    "test-pipe",
				Trigger: TriggerManual,
				Steps: []Step{
					{Name: "step1", Expression: "sum(clicks)"},
				},
			},
		},
		{
			name:    "missing name",
			pipe:    &Pipeline{Steps: []Step{{Name: "s1"}}},
			wantErr: ErrInvalidPipeline,
		},
		{
			name:    "no steps",
			pipe:    &Pipeline{Name: "empty", Steps: []Step{}},
			wantErr: ErrInvalidPipeline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.RegisterPipeline(tt.pipe)
			if tt.wantErr != nil {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEngine_DuplicatePipeline(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	p := &Pipeline{Name: "dup", Steps: []Step{{Name: "s1"}}}
	if err := e.RegisterPipeline(p); err != nil {
		t.Fatal(err)
	}
	if err := e.RegisterPipeline(p); err != ErrPipelineExists {
		t.Fatalf("expected ErrPipelineExists, got %v", err)
	}
}

func TestEngine_CyclicDependency(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	p := &Pipeline{
		Name: "cyclic",
		Steps: []Step{
			{Name: "a", DependsOn: []string{"b"}},
			{Name: "b", DependsOn: []string{"a"}},
		},
	}
	err := e.RegisterPipeline(p)
	if err != ErrCyclicDependency {
		t.Fatalf("expected ErrCyclicDependency, got %v", err)
	}
}

func TestEngine_TopologicalSort(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	p := &Pipeline{
		Name: "dag",
		Steps: []Step{
			{Name: "c", DependsOn: []string{"a", "b"}},
			{Name: "a"},
			{Name: "b", DependsOn: []string{"a"}},
		},
	}

	order, err := e.topologicalSort(p)
	if err != nil {
		t.Fatal(err)
	}

	// a must come before b and c; b must come before c
	indexOf := func(name string) int {
		for i, n := range order {
			if n == name {
				return i
			}
		}
		return -1
	}

	if indexOf("a") > indexOf("b") {
		t.Fatal("a should come before b")
	}
	if indexOf("b") > indexOf("c") {
		t.Fatal("b should come before c")
	}
}

func TestEngine_ExecutePipeline(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	callOrder := make([]string, 0)
	makeTransform := func(name string) TransformFunc {
		return func(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			callOrder = append(callOrder, name)
			return map[string]interface{}{"result": name}, nil
		}
	}

	p := &Pipeline{
		Name: "exec-test",
		Steps: []Step{
			{Name: "extract", Transform: makeTransform("extract")},
			{Name: "transform", DependsOn: []string{"extract"}, Transform: makeTransform("transform")},
			{Name: "load", DependsOn: []string{"transform"}, Transform: makeTransform("load")},
		},
	}

	if err := e.RegisterPipeline(p); err != nil {
		t.Fatal(err)
	}

	run, err := e.ExecutePipeline(context.Background(), "exec-test", TriggerManual)
	if err != nil {
		t.Fatal(err)
	}

	if run.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", run.Status)
	}
	if len(run.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(run.Steps))
	}
	if len(callOrder) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(callOrder))
	}
	if callOrder[0] != "extract" || callOrder[1] != "transform" || callOrder[2] != "load" {
		t.Fatalf("wrong execution order: %v", callOrder)
	}
}

func TestEngine_ExecuteStepFailure(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	p := &Pipeline{
		Name: "fail-test",
		Steps: []Step{
			{
				Name: "bad-step",
				Transform: func(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
					return nil, fmt.Errorf("step error")
				},
				RetryCount: 1,
			},
		},
	}

	if err := e.RegisterPipeline(p); err != nil {
		t.Fatal(err)
	}

	run, err := e.ExecutePipeline(context.Background(), "fail-test", TriggerManual)
	if err == nil {
		t.Fatal("expected error")
	}
	if run.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", run.Status)
	}
}

func TestEngine_CRUD(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	p := &Pipeline{
		Name:  "crud-pipe",
		Steps: []Step{{Name: "s1"}},
	}
	if err := e.RegisterPipeline(p); err != nil {
		t.Fatal(err)
	}

	got, err := e.GetPipeline("crud-pipe")
	if err != nil || got.Name != "crud-pipe" {
		t.Fatalf("get failed: %v", err)
	}

	list := e.ListPipelines()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	p.Steps = append(p.Steps, Step{Name: "s2"})
	if err := e.UpdatePipeline(p); err != nil {
		t.Fatal(err)
	}

	if err := e.DeletePipeline("crud-pipe"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.GetPipeline("crud-pipe"); err != ErrPipelineNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestEngine_Backfill(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	p := &Pipeline{
		Name:  "backfill-pipe",
		Steps: []Step{{Name: "s1", Expression: "count(*)"}},
	}
	if err := e.RegisterPipeline(p); err != nil {
		t.Fatal(err)
	}

	start := time.Now().Add(-2 * time.Hour)
	end := start.Add(2*time.Hour - time.Nanosecond)
	runs, err := e.Backfill(context.Background(), "backfill-pipe", start, end, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
}

func TestEngine_GetRuns(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	p := &Pipeline{
		Name:  "runs-pipe",
		Steps: []Step{{Name: "s1"}},
	}
	_ = e.RegisterPipeline(p)

	_, _ = e.ExecutePipeline(context.Background(), "runs-pipe", TriggerManual)
	_, _ = e.ExecutePipeline(context.Background(), "runs-pipe", TriggerManual)

	runs := e.GetRuns("runs-pipe")
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	run, err := e.GetRun("runs-pipe", runs[0].ID)
	if err != nil || run.ID != runs[0].ID {
		t.Fatalf("get run failed: %v", err)
	}

	_, err = e.GetRun("runs-pipe", "nonexistent")
	if err != ErrRunNotFound {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}

func TestEngine_RetryOnFailure(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	attempts := 0

	p := &Pipeline{
		Name: "retry-pipe",
		Steps: []Step{
			{
				Name:       "retry-step",
				RetryCount: 3,
				Transform: func(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
					attempts++
					if attempts < 3 {
						return nil, fmt.Errorf("transient error")
					}
					return map[string]interface{}{"ok": true}, nil
				},
			},
		},
	}
	_ = e.RegisterPipeline(p)

	run, err := e.ExecutePipeline(context.Background(), "retry-pipe", TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", run.Status)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

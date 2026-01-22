package featherqlv2

import (
	"testing"
)

func TestParse(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	result := e.Parse("SELECT avg(amount) AS avg_spend, count(*) AS txn_count FROM transactions")
	if !result.IsValid {
		t.Fatalf("expected valid parse, got errors: %v", result.Errors)
	}
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
	if len(result.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(result.Sources))
	}
}

func TestParseEmpty(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	result := e.Parse("")
	if result.IsValid {
		t.Error("expected invalid parse for empty query")
	}
}

func TestParseNoFrom(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	result := e.Parse("SELECT foo")
	if result.IsValid {
		t.Error("expected invalid parse without FROM")
	}
}

func TestCompile(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	pipeline, err := e.Compile("p1", "SELECT avg(amount) AS avg_spend FROM transactions WHERE user_id = '123'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline.ID != "p1" {
		t.Errorf("expected ID 'p1', got %q", pipeline.ID)
	}
	if len(pipeline.Steps) < 3 {
		t.Errorf("expected at least 3 execution steps, got %d", len(pipeline.Steps))
	}
}

func TestCompileWithWindow(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	pipeline, err := e.Compile("p2", "SELECT avg(amount) OVER (PARTITION BY user_id WINDOW '1h') AS avg_spend FROM transactions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasWindow := false
	for _, step := range pipeline.Steps {
		if step.Operation == "WINDOW" {
			hasWindow = true
			break
		}
	}
	if !hasWindow {
		t.Error("expected WINDOW step in execution plan")
	}
}

func TestExecute(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	result, err := e.Execute("SELECT count(*) AS total FROM events")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
}

func TestPipelineLifecycle(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	_, _ = e.Compile("test", "SELECT x FROM y")

	pipelines := e.ListPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}

	if err := e.DeletePipeline("test"); err != nil {
		t.Fatal(err)
	}

	if _, err := e.GetPipeline("test"); err != ErrPipelineNotFound {
		t.Errorf("expected ErrPipelineNotFound, got %v", err)
	}
}

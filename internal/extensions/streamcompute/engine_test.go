package streamcompute

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	stats := e.Stats()
	if stats.TotalPipelines != 0 {
		t.Errorf("expected 0 pipelines, got %d", stats.TotalPipelines)
	}
}

func TestCreateAndListPipelines(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())

	cfg := PipelineConfig{
		ID:          "test-pipeline",
		Description: "test tumbling window",
		Window: WindowConfig{
			Type: WindowTumbling,
			Size: 1 * time.Minute,
		},
		Aggregation: AggSum,
		GroupByKey:  true,
	}

	if err := e.CreatePipeline(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Duplicate should fail
	if err := e.CreatePipeline(cfg); err != ErrPipelineExists {
		t.Fatalf("expected ErrPipelineExists, got %v", err)
	}

	pipelines := e.ListPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].Config.ID != "test-pipeline" {
		t.Errorf("expected ID 'test-pipeline', got %q", pipelines[0].Config.ID)
	}
}

func TestPipelineLifecycle(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	cfg := PipelineConfig{
		ID: "lifecycle-test",
		Window: WindowConfig{
			Type: WindowTumbling,
			Size: 1 * time.Minute,
		},
		Aggregation: AggCount,
	}
	if err := e.CreatePipeline(cfg); err != nil {
		t.Fatal(err)
	}

	if err := e.StartPipeline("lifecycle-test"); err != nil {
		t.Fatal(err)
	}
	info, _ := e.GetPipeline("lifecycle-test")
	if info.Status != "running" {
		t.Errorf("expected running, got %s", info.Status)
	}

	if err := e.StopPipeline("lifecycle-test"); err != nil {
		t.Fatal(err)
	}
	info, _ = e.GetPipeline("lifecycle-test")
	if info.Status != "stopped" {
		t.Errorf("expected stopped, got %s", info.Status)
	}

	if err := e.DeletePipeline("lifecycle-test"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.GetPipeline("lifecycle-test"); err != ErrPipelineNotFound {
		t.Errorf("expected ErrPipelineNotFound, got %v", err)
	}
}

func TestTumblingWindowAggregation(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	cfg := PipelineConfig{
		ID: "tumbling-sum",
		Window: WindowConfig{
			Type: WindowTumbling,
			Size: 10 * time.Second,
		},
		Aggregation: AggSum,
		GroupByKey:  true,
	}
	if err := e.CreatePipeline(cfg); err != nil {
		t.Fatal(err)
	}
	if err := e.StartPipeline("tumbling-sum"); err != nil {
		t.Fatal(err)
	}

	base := time.Now()

	// Events within first window
	e.Ingest(Event{Key: "user1", Value: 10, Timestamp: base})
	e.Ingest(Event{Key: "user1", Value: 20, Timestamp: base.Add(5 * time.Second)})

	// Event that triggers window fire (past window end)
	results := e.Ingest(Event{Key: "user1", Value: 30, Timestamp: base.Add(11 * time.Second)})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Value != 30 { // sum of 10 + 20
		t.Errorf("expected sum 30, got %f", results[0].Value)
	}
	if results[0].Count != 2 {
		t.Errorf("expected count 2, got %d", results[0].Count)
	}
}

func TestInvalidWindowConfig(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	cfg := PipelineConfig{
		ID: "bad-window",
		Window: WindowConfig{
			Type: WindowSliding,
			Size: 10 * time.Second,
			// Missing slide interval
		},
		Aggregation: AggCount,
	}
	if err := e.CreatePipeline(cfg); err == nil {
		t.Fatal("expected error for missing slide interval")
	}
}

func TestEngineStats(t *testing.T) {
	e := NewEngine(DefaultEngineConfig())
	cfg := PipelineConfig{
		ID: "stats-test",
		Window: WindowConfig{
			Type: WindowTumbling,
			Size: 1 * time.Minute,
		},
		Aggregation: AggCount,
	}
	_ = e.CreatePipeline(cfg)
	_ = e.StartPipeline("stats-test")

	stats := e.Stats()
	if stats.TotalPipelines != 1 {
		t.Errorf("expected 1 pipeline, got %d", stats.TotalPipelines)
	}
	if stats.RunningPipelines != 1 {
		t.Errorf("expected 1 running, got %d", stats.RunningPipelines)
	}
}

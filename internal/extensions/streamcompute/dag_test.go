package streamcompute

import (
	"context"
	"testing"
	"time"
)

func TestDAGOrchestrator_CreateAndStart(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	orch := NewDAGOrchestrator(engine, 1000)

	cfg := DAGConfig{
		ID:   "test-dag",
		Name: "Test DAG",
		Nodes: []DAGNode{
			{ID: "source", Type: DAGNodeSource, Downstream: []string{"window"}},
			{ID: "window", Type: DAGNodeWindow, Upstream: []string{"source"}, Downstream: []string{"sink"}},
			{ID: "sink", Type: DAGNodeSink, Upstream: []string{"window"}},
		},
	}

	if err := orch.CreateDAG(cfg); err != nil {
		t.Fatal(err)
	}

	if err := orch.StartDAG(context.Background(), "test-dag"); err != nil {
		t.Fatal(err)
	}

	dags := orch.ListDAGs()
	if len(dags) != 1 {
		t.Fatalf("expected 1 DAG, got %d", len(dags))
	}
}

func TestDAGOrchestrator_CycleDetection(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	orch := NewDAGOrchestrator(engine, 1000)

	cfg := DAGConfig{
		ID: "cycle-dag",
		Nodes: []DAGNode{
			{ID: "a", Downstream: []string{"b"}},
			{ID: "b", Downstream: []string{"c"}},
			{ID: "c", Downstream: []string{"a"}}, // Cycle!
		},
	}

	err := orch.CreateDAG(cfg)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestDAGOrchestrator_Ingest(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	// Create a pipeline for the window node
	err := engine.CreatePipeline(PipelineConfig{
		ID:          "dag-pipeline",
		Window:      WindowConfig{Type: WindowTumbling, Size: time.Second},
		GroupByKey:  true,
		Aggregation: AggSum,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.StartPipeline("dag-pipeline"); err != nil {
		t.Fatal(err)
	}

	orch := NewDAGOrchestrator(engine, 1000)
	cfg := DAGConfig{
		ID: "ingest-dag",
		Nodes: []DAGNode{
			{ID: "source", Type: DAGNodeSource, Downstream: []string{"window"}},
			{ID: "window", Type: DAGNodeWindow, PipelineID: "dag-pipeline", Upstream: []string{"source"}, Downstream: []string{"sink"}},
			{ID: "sink", Type: DAGNodeSink, Upstream: []string{"window"}},
		},
	}
	if err := orch.CreateDAG(cfg); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartDAG(context.Background(), "ingest-dag"); err != nil {
		t.Fatal(err)
	}

	event := Event{Key: "user:1", Value: 10.0, Timestamp: time.Now()}
	_, err = orch.IngestToDAG("ingest-dag", event)
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}
}

func TestDAGOrchestrator_DuplicateNode(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	orch := NewDAGOrchestrator(engine, 1000)

	cfg := DAGConfig{
		ID: "dup-dag",
		Nodes: []DAGNode{
			{ID: "a", Type: DAGNodeSource},
			{ID: "a", Type: DAGNodeSink}, // duplicate
		},
	}

	err := orch.CreateDAG(cfg)
	if err == nil {
		t.Fatal("expected duplicate node error")
	}
}

package featherqlv2

import (
	"testing"
	"time"
)

func TestStreamPipelineSpec_ToStreamComputeConfig(t *testing.T) {
	spec := &StreamPipelineSpec{
		ID:            "test-1",
		SQL:           "SELECT COUNT(*) FROM events TUMBLING(5m)",
		SourceStream:  "events",
		GroupByKey:    "user_id",
		Aggregation:   "count",
		OutputFeature: "event_count",
		Window: &WindowSpec{
			Type: "tumbling",
			Size: 5 * time.Minute,
		},
	}

	cfg, err := spec.ToStreamComputeConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WindowType != "tumbling" {
		t.Errorf("expected tumbling, got %s", cfg.WindowType)
	}
	if cfg.WindowSize != 5*time.Minute {
		t.Errorf("expected 5m window, got %v", cfg.WindowSize)
	}
	if cfg.Aggregation != "count" {
		t.Errorf("expected count, got %s", cfg.Aggregation)
	}
	if cfg.OutputFeature != "event_count" {
		t.Errorf("expected event_count, got %s", cfg.OutputFeature)
	}
}

func TestStreamPipelineSpec_ToStreamComputeConfig_NoWindow(t *testing.T) {
	spec := &StreamPipelineSpec{
		ID:  "test-2",
		SQL: "SELECT * FROM events",
	}

	cfg, err := spec.ToStreamComputeConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WindowType != "tumbling" {
		t.Errorf("expected default tumbling, got %s", cfg.WindowType)
	}
	if cfg.Aggregation != "count" {
		t.Errorf("expected default count, got %s", cfg.Aggregation)
	}
}

func TestStreamCompiler_CompileToStreamCompute(t *testing.T) {
	compiler := NewStreamCompiler()
	cfg, err := compiler.CompileToStreamCompute("SELECT COUNT(*) as cnt FROM events GROUP BY user_id TUMBLING(10m)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WindowType != "tumbling" {
		t.Errorf("expected tumbling, got %s", cfg.WindowType)
	}
	if cfg.WindowSize != 10*time.Minute {
		t.Errorf("expected 10m, got %v", cfg.WindowSize)
	}
	if cfg.GroupByKey != "user_id" {
		t.Errorf("expected user_id, got %s", cfg.GroupByKey)
	}
}

package featherqlv2

import (
	"strings"
	"testing"
)

func TestCompileStreamingSelect(t *testing.T) {
	c := NewStreamCompiler()

	t.Run("tumbling window aggregation", func(t *testing.T) {
		spec, err := c.Compile("SELECT user_id, COUNT(*) AS cnt FROM clicks GROUP BY user_id TUMBLING(5m) EMIT CHANGES")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.ID == "" {
			t.Fatal("expected non-empty pipeline ID")
		}
		if spec.Window == nil {
			t.Fatal("expected window spec")
		}
		if spec.Window.Type != "tumbling" {
			t.Fatalf("expected tumbling window, got %s", spec.Window.Type)
		}
		if spec.GroupByKey != "user_id" {
			t.Fatalf("expected group by 'user_id', got %s", spec.GroupByKey)
		}
		if spec.Aggregation != "count" {
			t.Fatalf("expected aggregation 'count', got %s", spec.Aggregation)
		}
		if spec.OutputFeature != "cnt" {
			t.Fatalf("expected output feature 'cnt', got %s", spec.OutputFeature)
		}
	})

	t.Run("non-select rejected", func(t *testing.T) {
		_, err := c.Compile("CREATE STREAM foo (col1 STRING)")
		if err == nil {
			t.Fatal("expected error for non-SELECT statement")
		}
		if !strings.Contains(err.Error(), "only SELECT") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("default source stream", func(t *testing.T) {
		spec, err := c.Compile("SELECT * TUMBLING(1m) EMIT FINAL")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.SourceStream != "default" {
			t.Fatalf("expected default source stream, got %s", spec.SourceStream)
		}
	})
}

func TestPipelineManagement(t *testing.T) {
	c := NewStreamCompiler()

	spec1, err := c.Compile("SELECT user_id, SUM(amount) AS total FROM orders GROUP BY user_id TUMBLING(10m) EMIT CHANGES")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec2, err := c.Compile("SELECT region, AVG(latency) AS avg_lat FROM requests GROUP BY region SLIDING(5m) EMIT FINAL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("list", func(t *testing.T) {
		pipelines := c.ListPipelines()
		if len(pipelines) != 2 {
			t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
		}
	})

	t.Run("get", func(t *testing.T) {
		got, err := c.GetPipeline(spec1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != spec1.ID {
			t.Fatalf("expected ID %s, got %s", spec1.ID, got.ID)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		_, err := c.GetPipeline("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent pipeline")
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := c.DeletePipeline(spec2.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pipelines := c.ListPipelines()
		if len(pipelines) != 1 {
			t.Fatalf("expected 1 pipeline after delete, got %d", len(pipelines))
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		err := c.DeletePipeline("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent pipeline")
		}
	})
}

func TestExtractAggFunction(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"COUNT(*)", "count"},
		{"SUM(amount)", "sum"},
		{"AVG(latency)", "avg"},
		{"MIN(price)", "min"},
		{"MAX(price)", "max"},
		{"user_id", ""},
	}
	for _, tt := range tests {
		got := extractAggFunction(tt.expr)
		if got != tt.want {
			t.Errorf("extractAggFunction(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

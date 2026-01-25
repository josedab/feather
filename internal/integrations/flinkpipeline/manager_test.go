package flinkpipeline

import (
	"errors"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestCreatePipeline(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	p, err := m.CreatePipeline(Pipeline{
		ID:      "test-pipeline",
		Name:    "Test Pipeline",
		Runtime: RuntimeKafkaStreams,
		Source:  Source{Type: "kafka", Topic: "events"},
		Sink:    Sink{Type: "feather", FeatureGroup: "user_features"},
		Stages: []TransformStage{
			{Name: "count-events", Type: "aggregate", Aggregation: AggCount, KeyBy: "user_id"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != StatusCreated {
		t.Errorf("expected status created, got %s", p.Status)
	}
	if p.Parallelism != 4 {
		t.Errorf("expected default parallelism 4, got %d", p.Parallelism)
	}
}

func TestCreateDuplicatePipeline(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})
	_, err := m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1 dup",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})
	if !errors.Is(err, ErrPipelineExists) {
		t.Fatalf("expected ErrPipelineExists, got %v", err)
	}
}

func TestCreateInvalidPipeline(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, err := m.CreatePipeline(Pipeline{})
	if !errors.Is(err, ErrInvalidPipeline) {
		t.Fatalf("expected ErrInvalidPipeline, got %v", err)
	}
}

func TestStartStopPipeline(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})

	if err := m.StartPipeline("p1"); err != nil {
		t.Fatal(err)
	}
	p, _ := m.GetPipeline("p1")
	if p.Status != StatusRunning {
		t.Errorf("expected running, got %s", p.Status)
	}

	if err := m.StopPipeline("p1"); err != nil {
		t.Fatal(err)
	}
	p, _ = m.GetPipeline("p1")
	if p.Status != StatusStopped {
		t.Errorf("expected stopped, got %s", p.Status)
	}
}

func TestDeleteRunningPipeline(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})
	_ = m.StartPipeline("p1")

	err := m.DeletePipeline("p1")
	if !errors.Is(err, ErrPipelineRunning) {
		t.Fatalf("expected ErrPipelineRunning, got %v", err)
	}
}

func TestDeletePipeline(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})

	if err := m.DeletePipeline("p1"); err != nil {
		t.Fatal(err)
	}
	_, err := m.GetPipeline("p1")
	if !errors.Is(err, ErrPipelineNotFound) {
		t.Fatalf("expected ErrPipelineNotFound, got %v", err)
	}
}

func TestListPipelines(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p2", Name: "P2",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})

	pipelines := m.ListPipelines()
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}
}

func TestIngestEvent(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})
	_ = m.StartPipeline("p1")

	err := m.IngestEvent("p1", map[string]interface{}{"user_id": "u1", "amount": 10.5})
	if err != nil {
		t.Fatal(err)
	}

	stats, err := m.GetPipelineStats("p1")
	if err != nil {
		t.Fatal(err)
	}
	if stats.EventsIn != 1 {
		t.Errorf("expected 1 event in, got %d", stats.EventsIn)
	}
}

func TestPipelineNotFound(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, err := m.GetPipeline("nonexistent")
	if !errors.Is(err, ErrPipelineNotFound) {
		t.Fatalf("expected ErrPipelineNotFound, got %v", err)
	}
}

func TestManagerStats(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, _ = m.CreatePipeline(Pipeline{
		ID: "p1", Name: "P1",
		Source: Source{Type: "kafka"}, Sink: Sink{Type: "feather"},
	})
	_ = m.StartPipeline("p1")

	stats := m.Stats()
	if stats.TotalPipelines != 1 {
		t.Errorf("expected 1 total pipeline, got %d", stats.TotalPipelines)
	}
	if stats.RunningPipelines != 1 {
		t.Errorf("expected 1 running pipeline, got %d", stats.RunningPipelines)
	}
}

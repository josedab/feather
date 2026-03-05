package streamingcdc

import (
	"context"
	"testing"
	"time"
)

func TestPipelineLifecycle(t *testing.T) {
	config := DefaultPipelineConfig()
	config.ID = "test-pipeline"
	config.SourceID = "src-1"
	config.TargetFeatureGroup = "user_features"

	p := NewPipeline(config)
	if p.State() != StateStopped {
		t.Fatalf("expected stopped, got %s", p.State())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if p.State() != StateRunning {
		t.Fatalf("expected running, got %s", p.State())
	}

	// Ingest records
	for i := 0; i < 10; i++ {
		err := p.Ingest(ChangeRecord{
			SourceID:  "src-1",
			Operation: "INSERT",
			EntityID:  "user:1",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	stats := p.Stats()
	if stats.RecordsIngested != 10 {
		t.Errorf("expected 10 ingested, got %d", stats.RecordsIngested)
	}
	if stats.RecordsProcessed < 10 {
		t.Errorf("expected >=10 processed, got %d", stats.RecordsProcessed)
	}

	p.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestManagerCreateAndList(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	info, err := mgr.CreatePipeline(PipelineConfig{
		ID:                 "p1",
		Name:               "Test Pipeline",
		SourceID:           "src-1",
		TargetFeatureGroup: "features",
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "p1" {
		t.Errorf("expected id p1, got %s", info.ID)
	}

	// Duplicate
	_, err = mgr.CreatePipeline(PipelineConfig{
		ID:                 "p1",
		SourceID:           "src-1",
		TargetFeatureGroup: "features",
	})
	if err == nil {
		t.Fatal("expected error for duplicate pipeline")
	}

	// List
	pipelines := mgr.ListPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}

	// Delete
	if err := mgr.DeletePipeline("p1"); err != nil {
		t.Fatal(err)
	}
	if len(mgr.ListPipelines()) != 0 {
		t.Fatal("expected 0 pipelines after delete")
	}
}

func TestPipelineBackpressure(t *testing.T) {
	config := DefaultPipelineConfig()
	config.ID = "bp-test"
	config.SourceID = "src-1"
	config.TargetFeatureGroup = "features"
	config.BufferSize = 5

	p := NewPipeline(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Fill buffer beyond capacity
	ingested, dropped := p.IngestBatch(make([]ChangeRecord, 100))
	if ingested+dropped != 100 {
		t.Errorf("expected total 100, got %d+%d", ingested, dropped)
	}

	p.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestManagerValidation(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	// Missing ID
	_, err := mgr.CreatePipeline(PipelineConfig{SourceID: "s", TargetFeatureGroup: "f"})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	// Missing source
	_, err = mgr.CreatePipeline(PipelineConfig{ID: "p", TargetFeatureGroup: "f"})
	if err == nil {
		t.Fatal("expected error for missing source")
	}

	// Missing target
	_, err = mgr.CreatePipeline(PipelineConfig{ID: "p", SourceID: "s"})
	if err == nil {
		t.Fatal("expected error for missing target")
	}
}

package embeddingmgmt

import (
	"errors"
	"testing"
)

func TestNewVectorDriftDetector(t *testing.T) {
	d := NewVectorDriftDetector(DefaultVectorDriftConfig())
	if d == nil {
		t.Fatal("expected non-nil drift detector")
	}
}

func TestDriftSetReference(t *testing.T) {
	d := NewVectorDriftDetector(DefaultVectorDriftConfig())

	err := d.SetReference("col", "m1", [][]float64{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	monitored := d.ListMonitored()
	if len(monitored) != 1 {
		t.Fatalf("expected 1 monitored, got %d", len(monitored))
	}
	if monitored[0].Collection != "col" {
		t.Errorf("expected collection 'col', got %s", monitored[0].Collection)
	}
}

func TestDriftSetReferenceEmpty(t *testing.T) {
	d := NewVectorDriftDetector(DefaultVectorDriftConfig())

	err := d.SetReference("col", "m1", nil)
	if err == nil {
		t.Error("expected error for empty reference vectors")
	}
}

func TestDriftSetReferenceDimensionMismatch(t *testing.T) {
	d := NewVectorDriftDetector(DefaultVectorDriftConfig())

	err := d.SetReference("col", "m1", [][]float64{
		{1, 0, 0},
		{0, 1}, // wrong dimensions
	})
	if !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("expected ErrDimensionMismatch, got %v", err)
	}
}

func TestDriftCheckNoDrift(t *testing.T) {
	cfg := VectorDriftConfig{
		WindowSize:     100,
		DriftThreshold: 0.1,
		MinSamples:     5,
	}
	d := NewVectorDriftDetector(cfg)

	ref := [][]float64{
		{1, 0, 0},
		{0.9, 0.1, 0},
		{0.95, 0.05, 0},
	}
	_ = d.SetReference("col", "m1", ref)

	// Record similar vectors
	for i := 0; i < 10; i++ {
		d.RecordEmbedding("col", "m1", []float32{0.95, 0.05, 0})
	}

	status, err := d.CheckDrift("col", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if status.IsDrifting {
		t.Errorf("expected no drift, got drift=%f", status.CurrentDrift)
	}
	if status.SampleCount != 10 {
		t.Errorf("expected 10 samples, got %d", status.SampleCount)
	}
}

func TestDriftCheckWithDrift(t *testing.T) {
	cfg := VectorDriftConfig{
		WindowSize:     100,
		DriftThreshold: 0.1,
		MinSamples:     5,
	}
	d := NewVectorDriftDetector(cfg)

	ref := [][]float64{
		{1, 0, 0},
		{1, 0, 0},
		{1, 0, 0},
	}
	_ = d.SetReference("col", "m1", ref)

	// Record very different vectors
	for i := 0; i < 10; i++ {
		d.RecordEmbedding("col", "m1", []float32{0, 0, 1})
	}

	status, err := d.CheckDrift("col", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if !status.IsDrifting {
		t.Errorf("expected drift, got drift=%f (threshold=%f)", status.CurrentDrift, status.Threshold)
	}
	if status.CurrentDrift <= cfg.DriftThreshold {
		t.Errorf("expected drift > %f, got %f", cfg.DriftThreshold, status.CurrentDrift)
	}
}

func TestDriftCheckNoReference(t *testing.T) {
	d := NewVectorDriftDetector(DefaultVectorDriftConfig())

	_, err := d.CheckDrift("nonexistent", "m1")
	if !errors.Is(err, ErrNoReference) {
		t.Fatalf("expected ErrNoReference, got %v", err)
	}
}

func TestDriftCheckInsufficientSamples(t *testing.T) {
	cfg := VectorDriftConfig{
		WindowSize:     100,
		DriftThreshold: 0.1,
		MinSamples:     50,
	}
	d := NewVectorDriftDetector(cfg)

	_ = d.SetReference("col", "m1", [][]float64{{1, 0, 0}})

	// Only add a few samples
	for i := 0; i < 5; i++ {
		d.RecordEmbedding("col", "m1", []float32{1, 0, 0})
	}

	_, err := d.CheckDrift("col", "m1")
	if err == nil {
		t.Error("expected error for insufficient samples")
	}
}

func TestDriftSlidingWindow(t *testing.T) {
	cfg := VectorDriftConfig{
		WindowSize:     5,
		DriftThreshold: 0.1,
		MinSamples:     3,
	}
	d := NewVectorDriftDetector(cfg)

	_ = d.SetReference("col", "m1", [][]float64{{1, 0, 0}})

	// Fill window beyond capacity
	for i := 0; i < 10; i++ {
		d.RecordEmbedding("col", "m1", []float32{0.9, 0.1, 0})
	}

	status, err := d.CheckDrift("col", "m1")
	if err != nil {
		t.Fatal(err)
	}
	// Window should keep only last 5
	if status.SampleCount != 5 {
		t.Errorf("expected 5 samples (window size), got %d", status.SampleCount)
	}
}

func TestDriftRecordIgnoresUnmonitored(t *testing.T) {
	d := NewVectorDriftDetector(DefaultVectorDriftConfig())

	// Should not panic
	d.RecordEmbedding("unmonitored", "m1", []float32{1, 0, 0})

	monitored := d.ListMonitored()
	if len(monitored) != 0 {
		t.Errorf("expected 0 monitored, got %d", len(monitored))
	}
}

func TestDriftHistory(t *testing.T) {
	cfg := VectorDriftConfig{
		WindowSize:     100,
		DriftThreshold: 0.1,
		MinSamples:     3,
	}
	d := NewVectorDriftDetector(cfg)

	_ = d.SetReference("col", "m1", [][]float64{{1, 0, 0}})

	for i := 0; i < 5; i++ {
		d.RecordEmbedding("col", "m1", []float32{0.9, 0.1, 0})
	}

	// Check drift twice to build history
	_, _ = d.CheckDrift("col", "m1")
	status, _ := d.CheckDrift("col", "m1")

	if len(status.DriftHistory) != 2 {
		t.Errorf("expected 2 history points, got %d", len(status.DriftHistory))
	}
}

func TestDriftListMonitoredMultiple(t *testing.T) {
	d := NewVectorDriftDetector(DefaultVectorDriftConfig())

	_ = d.SetReference("col-a", "m1", [][]float64{{1, 0, 0}})
	_ = d.SetReference("col-b", "m2", [][]float64{{0, 1, 0}})

	monitored := d.ListMonitored()
	if len(monitored) != 2 {
		t.Errorf("expected 2 monitored, got %d", len(monitored))
	}
}

func TestDriftDefaultConfig(t *testing.T) {
	cfg := DefaultVectorDriftConfig()
	if cfg.WindowSize != 1000 {
		t.Errorf("expected WindowSize 1000, got %d", cfg.WindowSize)
	}
	if cfg.DriftThreshold != 0.1 {
		t.Errorf("expected DriftThreshold 0.1, got %f", cfg.DriftThreshold)
	}
	if cfg.MinSamples != 50 {
		t.Errorf("expected MinSamples 50, got %d", cfg.MinSamples)
	}
}

func TestCosineDistance(t *testing.T) {
	// Identical vectors should have 0 distance
	d := cosineDistance([]float64{1, 0, 0}, []float64{1, 0, 0})
	if d > 1e-10 {
		t.Errorf("expected ~0 distance for identical vectors, got %f", d)
	}

	// Orthogonal vectors should have distance 1
	d = cosineDistance([]float64{1, 0, 0}, []float64{0, 1, 0})
	if d < 0.99 || d > 1.01 {
		t.Errorf("expected ~1 distance for orthogonal vectors, got %f", d)
	}

	// Empty vectors
	d = cosineDistance(nil, nil)
	if d != 1.0 {
		t.Errorf("expected 1.0 for empty vectors, got %f", d)
	}
}

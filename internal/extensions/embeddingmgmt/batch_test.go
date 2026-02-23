package embeddingmgmt

import (
	"errors"
	"testing"
	"time"
)

func newTestManagerWithCollection(dims int) (*Manager, string) {
	m := NewManager(DefaultManagerConfig())
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: dims})
	_, _ = m.CreateCollection("test-col", "m1", nil)
	return m, "test-col"
}

func TestNewBatchProcessor(t *testing.T) {
	m, _ := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, DefaultBatchConfig())
	if bp == nil {
		t.Fatal("expected non-nil batch processor")
	}
}

func TestBatchSubmitAndComplete(t *testing.T) {
	m, col := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, BatchConfig{
		MaxBatchSize:  10,
		Concurrency:   2,
		RetryAttempts: 1,
	})

	items := []BatchItem{
		{ID: "e1", Vector: []float64{1, 0, 0}},
		{ID: "e2", Vector: []float64{0, 1, 0}},
		{ID: "e3", Vector: []float64{0, 0, 1}},
	}

	job, err := bp.Submit("m1", col, items)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.TotalItems != 3 {
		t.Errorf("expected 3 total items, got %d", job.TotalItems)
	}

	// Wait for completion
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, _ := bp.GetJob(job.ID)
		if j.Status == BatchCompleted || j.Status == BatchFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	j, _ := bp.GetJob(job.ID)
	if j.Status != BatchCompleted {
		t.Errorf("expected completed, got %s", j.Status)
	}
	if j.Processed != 3 {
		t.Errorf("expected 3 processed, got %d", j.Processed)
	}
	if j.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", j.Failed)
	}

	// Verify embeddings were stored
	for _, item := range items {
		emb, err := m.Get(col, item.ID)
		if err != nil {
			t.Errorf("embedding %s not found: %v", item.ID, err)
		}
		if emb == nil {
			t.Errorf("expected non-nil embedding for %s", item.ID)
		}
	}
}

func TestBatchSubmitEmptyItems(t *testing.T) {
	m, col := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, DefaultBatchConfig())

	_, err := bp.Submit("m1", col, nil)
	if !errors.Is(err, ErrEmptyBatch) {
		t.Fatalf("expected ErrEmptyBatch, got %v", err)
	}
}

func TestBatchSubmitInvalidCollection(t *testing.T) {
	m, _ := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, DefaultBatchConfig())

	_, err := bp.Submit("m1", "nonexistent", []BatchItem{{ID: "e1", Vector: []float64{1, 0, 0}}})
	if !errors.Is(err, ErrCollectionNotFound) {
		t.Fatalf("expected ErrCollectionNotFound, got %v", err)
	}
}

func TestBatchGetJobNotFound(t *testing.T) {
	m, _ := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, DefaultBatchConfig())

	_, err := bp.GetJob("nonexistent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestBatchListJobs(t *testing.T) {
	m, col := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, BatchConfig{
		MaxBatchSize:  10,
		Concurrency:   1,
		RetryAttempts: 1,
	})

	_, _ = bp.Submit("m1", col, []BatchItem{{ID: "e1", Vector: []float64{1, 0, 0}}})
	_, _ = bp.Submit("m1", col, []BatchItem{{ID: "e2", Vector: []float64{0, 1, 0}}})

	// Wait briefly for jobs to start
	time.Sleep(100 * time.Millisecond)

	jobs := bp.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestBatchCancelJob(t *testing.T) {
	m, col := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, BatchConfig{
		MaxBatchSize:  1,
		Concurrency:   1,
		RetryAttempts: 1,
	})

	// Submit a larger job to increase chance of cancellation
	items := make([]BatchItem, 100)
	for i := range items {
		items[i] = BatchItem{ID: "e" + string(rune('0'+i%10)), Vector: []float64{1, 0, 0}}
	}

	job, err := bp.Submit("m1", col, items)
	if err != nil {
		t.Fatal(err)
	}

	err = bp.CancelJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for job to register cancellation
	time.Sleep(100 * time.Millisecond)

	j, _ := bp.GetJob(job.ID)
	if j.Status != BatchCanceled {
		t.Errorf("expected canceled, got %s", j.Status)
	}
}

func TestBatchCancelNonexistentJob(t *testing.T) {
	m, _ := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, DefaultBatchConfig())

	err := bp.CancelJob("nonexistent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestBatchWithDimensionMismatch(t *testing.T) {
	m, col := newTestManagerWithCollection(3)
	bp := NewBatchProcessor(m, BatchConfig{
		MaxBatchSize:  10,
		Concurrency:   1,
		RetryAttempts: 1,
	})

	items := []BatchItem{
		{ID: "e1", Vector: []float64{1, 0}}, // wrong dimensions
	}

	job, err := bp.Submit("m1", col, items)
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, _ := bp.GetJob(job.ID)
		if j.Status == BatchCompleted || j.Status == BatchFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	j, _ := bp.GetJob(job.ID)
	if j.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", j.Failed)
	}
	if len(j.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(j.Errors))
	}
}

func TestBatchDefaultConfig(t *testing.T) {
	cfg := DefaultBatchConfig()
	if cfg.MaxBatchSize != 1000 {
		t.Errorf("expected MaxBatchSize 1000, got %d", cfg.MaxBatchSize)
	}
	if cfg.Concurrency != 4 {
		t.Errorf("expected Concurrency 4, got %d", cfg.Concurrency)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("expected RetryAttempts 3, got %d", cfg.RetryAttempts)
	}
}

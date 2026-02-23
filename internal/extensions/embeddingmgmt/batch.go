package embeddingmgmt

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BatchConfig configures the batch processor.
type BatchConfig struct {
	MaxBatchSize  int
	Concurrency   int
	RetryAttempts int
}

// DefaultBatchConfig returns sensible defaults.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:  1000,
		Concurrency:   4,
		RetryAttempts: 3,
	}
}

// BatchStatus represents the state of a batch job.
type BatchStatus string

// Batch status values.
const (
	BatchPending   BatchStatus = "pending"
	BatchRunning   BatchStatus = "running"
	BatchCompleted BatchStatus = "completed"
	BatchFailed    BatchStatus = "failed"
	BatchCanceled  BatchStatus = "canceled"
)

// BatchItem represents a single item to embed.
type BatchItem struct {
	ID     string    `json:"id"`
	Vector []float64 `json:"vector"`
}

// BatchJob tracks the progress of a batch embedding job.
type BatchJob struct {
	ID          string      `json:"id"`
	ModelID     string      `json:"model_id"`
	Collection  string      `json:"collection"`
	Status      BatchStatus `json:"status"`
	TotalItems  int         `json:"total_items"`
	Processed   int         `json:"processed"`
	Failed      int         `json:"failed"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Errors      []string    `json:"errors,omitempty"`

	items    []BatchItem
	cancelCh chan struct{}
}

// Batch processor errors.
var (
	ErrJobNotFound = errors.New("batch job not found")
	ErrJobCanceled = errors.New("batch job canceled")
	ErrEmptyBatch  = errors.New("batch items cannot be empty")
)

// BatchProcessor manages batch embedding jobs.
type BatchProcessor struct {
	mu   sync.RWMutex
	mgr  *Manager
	cfg  BatchConfig
	jobs map[string]*BatchJob
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(mgr *Manager, cfg BatchConfig) *BatchProcessor {
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = DefaultBatchConfig().MaxBatchSize
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = DefaultBatchConfig().Concurrency
	}
	if cfg.RetryAttempts <= 0 {
		cfg.RetryAttempts = DefaultBatchConfig().RetryAttempts
	}
	return &BatchProcessor{
		mgr:  mgr,
		cfg:  cfg,
		jobs: make(map[string]*BatchJob),
	}
}

// Submit creates and starts a batch embedding job.
func (bp *BatchProcessor) Submit(modelID, collection string, items []BatchItem) (*BatchJob, error) {
	if len(items) == 0 {
		return nil, ErrEmptyBatch
	}

	// Validate collection exists
	if _, err := bp.mgr.GetCollection(collection); err != nil {
		return nil, err
	}

	job := &BatchJob{
		ID:         uuid.New().String(),
		ModelID:    modelID,
		Collection: collection,
		Status:     BatchPending,
		TotalItems: len(items),
		StartedAt:  time.Now(),
		items:      items,
		cancelCh:   make(chan struct{}),
	}

	bp.mu.Lock()
	bp.jobs[job.ID] = job
	bp.mu.Unlock()

	go bp.processJob(job)

	return job, nil
}

// processJob runs the batch job with concurrency control.
func (bp *BatchProcessor) processJob(job *BatchJob) {
	bp.mu.Lock()
	job.Status = BatchRunning
	bp.mu.Unlock()

	sem := make(chan struct{}, bp.cfg.Concurrency)

	// Process in chunks up to MaxBatchSize
	for i := 0; i < len(job.items); i += bp.cfg.MaxBatchSize {
		select {
		case <-job.cancelCh:
			bp.mu.Lock()
			job.Status = BatchCanceled
			bp.mu.Unlock()
			return
		default:
		}

		end := i + bp.cfg.MaxBatchSize
		if end > len(job.items) {
			end = len(job.items)
		}
		chunk := job.items[i:end]

		var wg sync.WaitGroup
		for _, item := range chunk {
			select {
			case <-job.cancelCh:
				bp.mu.Lock()
				job.Status = BatchCanceled
				bp.mu.Unlock()
				return
			default:
			}

			wg.Add(1)
			sem <- struct{}{}
			go func(it BatchItem) {
				defer wg.Done()
				defer func() { <-sem }()

				bp.processItem(job, it)
			}(item)
		}
		wg.Wait()
	}

	bp.mu.Lock()
	now := time.Now()
	job.CompletedAt = &now
	if job.Failed > 0 && job.Failed == job.TotalItems {
		job.Status = BatchFailed
	} else {
		job.Status = BatchCompleted
	}
	bp.mu.Unlock()
}

// processItem upserts a single item with retry logic.
func (bp *BatchProcessor) processItem(job *BatchJob, item BatchItem) {
	var lastErr error
	for attempt := 0; attempt < bp.cfg.RetryAttempts; attempt++ {
		emb := Embedding{
			ID:     item.ID,
			Vector: item.Vector,
		}
		if err := bp.mgr.Upsert(job.Collection, emb); err != nil {
			lastErr = err
			continue
		}
		bp.mu.Lock()
		job.Processed++
		bp.mu.Unlock()
		return
	}

	bp.mu.Lock()
	job.Failed++
	if lastErr != nil {
		job.Errors = append(job.Errors, fmt.Sprintf("item %s: %v", item.ID, lastErr))
	}
	bp.mu.Unlock()
}

// GetJob returns a snapshot of a batch job's status.
func (bp *BatchProcessor) GetJob(id string) (*BatchJob, error) {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	job, exists := bp.jobs[id]
	if !exists {
		return nil, ErrJobNotFound
	}
	// Return a copy to avoid races with the processing goroutine.
	snapshot := *job
	if job.Errors != nil {
		snapshot.Errors = make([]string, len(job.Errors))
		copy(snapshot.Errors, job.Errors)
	}
	return &snapshot, nil
}

// ListJobs returns all batch jobs.
func (bp *BatchProcessor) ListJobs() []BatchJob {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	result := make([]BatchJob, 0, len(bp.jobs))
	for _, job := range bp.jobs {
		result = append(result, *job)
	}
	return result
}

// CancelJob cancels a running batch job.
func (bp *BatchProcessor) CancelJob(id string) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	job, exists := bp.jobs[id]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status != BatchPending && job.Status != BatchRunning {
		return fmt.Errorf("cannot cancel job with status %s", job.Status)
	}

	close(job.cancelCh)
	job.Status = BatchCanceled
	return nil
}

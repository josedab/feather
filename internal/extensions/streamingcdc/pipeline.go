package streamingcdc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PipelineState represents the lifecycle state of a streaming pipeline.
type PipelineState string

const (
	StateStopped  PipelineState = "stopped"
	StateStarting PipelineState = "starting"
	StateRunning  PipelineState = "running"
	StatePaused   PipelineState = "paused"
	StateDraining PipelineState = "draining"
	StateFailed   PipelineState = "failed"
)

// PipelineConfig configures a streaming CDC pipeline.
type PipelineConfig struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	SourceID           string        `json:"source_id"`
	TargetFeatureGroup string        `json:"target_feature_group"`
	BufferSize         int           `json:"buffer_size"`
	BatchSize          int           `json:"batch_size"`
	FlushInterval      time.Duration `json:"flush_interval_ns"`
	MaxLatency         time.Duration `json:"max_latency_ns"`
	Parallelism        int           `json:"parallelism"`
	EnableWatermarks   bool          `json:"enable_watermarks"`
	WatermarkInterval  time.Duration `json:"watermark_interval_ns"`
	RetryAttempts      int           `json:"retry_attempts"`
	RetryBackoff       time.Duration `json:"retry_backoff_ns"`
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BufferSize:        10000,
		BatchSize:         100,
		FlushInterval:     time.Second,
		MaxLatency:        5 * time.Second,
		Parallelism:       4,
		EnableWatermarks:  true,
		WatermarkInterval: time.Second,
		RetryAttempts:     3,
		RetryBackoff:      100 * time.Millisecond,
	}
}

// ChangeRecord represents a single change flowing through the pipeline.
type ChangeRecord struct {
	SourceID  string                 `json:"source_id"`
	Operation string                 `json:"operation"`
	EntityID  string                 `json:"entity_id"`
	Table     string                 `json:"table"`
	Before    map[string]interface{} `json:"before,omitempty"`
	After     map[string]interface{} `json:"after,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	LSN       int64                  `json:"lsn"`
	Partition int                    `json:"partition"`
}

// Watermark tracks event-time progress through the pipeline.
type Watermark struct {
	SourceID  string    `json:"source_id"`
	Timestamp time.Time `json:"timestamp"`
	LSN       int64     `json:"lsn"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PipelineStats holds streaming pipeline statistics.
type PipelineStats struct {
	RecordsIngested   int64         `json:"records_ingested"`
	RecordsProcessed  int64         `json:"records_processed"`
	RecordsFailed     int64         `json:"records_failed"`
	BatchesProcessed  int64         `json:"batches_processed"`
	CurrentLag        time.Duration `json:"current_lag_ns"`
	CurrentWatermark  time.Time     `json:"current_watermark"`
	BackpressureCount int64         `json:"backpressure_count"`
	LastProcessedAt   time.Time     `json:"last_processed_at"`
	Uptime            time.Duration `json:"uptime_ns"`
}

// TransformFunc transforms a change record before materialization.
type TransformFunc func(record *ChangeRecord) (*ChangeRecord, error)

// Pipeline manages a streaming CDC materialization pipeline.
type Pipeline struct {
	config     PipelineConfig
	state      PipelineState
	buffer     chan ChangeRecord
	transforms []TransformFunc
	watermarks map[string]*Watermark
	stats      PipelineStats
	startedAt  time.Time
	cancel     context.CancelFunc
	mu         sync.RWMutex

	ingested  atomic.Int64
	processed atomic.Int64
	failed    atomic.Int64
	batches   atomic.Int64
	backpress atomic.Int64
}

// NewPipeline creates a new streaming CDC pipeline.
func NewPipeline(config PipelineConfig) *Pipeline {
	if config.BufferSize <= 0 {
		config.BufferSize = 10000
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.Parallelism <= 0 {
		config.Parallelism = 4
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = time.Second
	}
	return &Pipeline{
		config:     config,
		state:      StateStopped,
		buffer:     make(chan ChangeRecord, config.BufferSize),
		watermarks: make(map[string]*Watermark),
	}
}

// Start begins processing the pipeline.
func (p *Pipeline) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.state == StateRunning {
		p.mu.Unlock()
		return fmt.Errorf("pipeline %s already running", p.config.ID)
	}
	ctx, p.cancel = context.WithCancel(ctx)
	p.state = StateRunning
	p.startedAt = time.Now()
	p.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < p.config.Parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.processLoop(ctx)
		}()
	}

	go func() {
		wg.Wait()
		p.mu.Lock()
		if p.state == StateRunning || p.state == StateDraining {
			p.state = StateStopped
		}
		p.mu.Unlock()
	}()

	return nil
}

// Stop gracefully stops the pipeline.
func (p *Pipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.state = StateDraining
		p.cancel()
	}
}

// Ingest adds a change record to the pipeline buffer.
func (p *Pipeline) Ingest(record ChangeRecord) error {
	p.mu.RLock()
	if p.state != StateRunning {
		p.mu.RUnlock()
		return fmt.Errorf("pipeline %s not running (state: %s)", p.config.ID, p.state)
	}
	p.mu.RUnlock()

	select {
	case p.buffer <- record:
		p.ingested.Add(1)
		return nil
	default:
		p.backpress.Add(1)
		return fmt.Errorf("pipeline %s buffer full (backpressure)", p.config.ID)
	}
}

// IngestBatch adds multiple records to the pipeline.
func (p *Pipeline) IngestBatch(records []ChangeRecord) (int, int) {
	ingested, dropped := 0, 0
	for _, r := range records {
		if err := p.Ingest(r); err != nil {
			dropped++
		} else {
			ingested++
		}
	}
	return ingested, dropped
}

// AddTransform adds a transform function to the pipeline.
func (p *Pipeline) AddTransform(fn TransformFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.transforms = append(p.transforms, fn)
}

func (p *Pipeline) processLoop(ctx context.Context) {
	batch := make([]ChangeRecord, 0, p.config.BatchSize)
	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				p.processBatch(batch)
			}
			return
		case record := <-p.buffer:
			transformed, err := p.applyTransforms(record)
			if err != nil {
				p.failed.Add(1)
				continue
			}
			batch = append(batch, *transformed)
			if len(batch) >= p.config.BatchSize {
				p.processBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				p.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (p *Pipeline) applyTransforms(record ChangeRecord) (*ChangeRecord, error) {
	p.mu.RLock()
	transforms := p.transforms
	p.mu.RUnlock()

	current := &record
	for _, fn := range transforms {
		result, err := fn(current)
		if err != nil {
			return nil, err
		}
		current = result
	}
	return current, nil
}

func (p *Pipeline) processBatch(records []ChangeRecord) {
	for _, r := range records {
		p.processed.Add(1)
		p.updateWatermark(r.SourceID, r.Timestamp, r.LSN)
	}
	p.batches.Add(1)
	p.mu.Lock()
	p.stats.LastProcessedAt = time.Now()
	p.mu.Unlock()
}

func (p *Pipeline) updateWatermark(sourceID string, ts time.Time, lsn int64) {
	if !p.config.EnableWatermarks {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	wm, exists := p.watermarks[sourceID]
	if !exists || ts.After(wm.Timestamp) {
		p.watermarks[sourceID] = &Watermark{
			SourceID:  sourceID,
			Timestamp: ts,
			LSN:       lsn,
			UpdatedAt: time.Now(),
		}
	}
}

// State returns the current pipeline state.
func (p *Pipeline) State() PipelineState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// Config returns the pipeline config.
func (p *Pipeline) Config() PipelineConfig {
	return p.config
}

// Stats returns current pipeline statistics.
func (p *Pipeline) Stats() PipelineStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := p.stats
	stats.RecordsIngested = p.ingested.Load()
	stats.RecordsProcessed = p.processed.Load()
	stats.RecordsFailed = p.failed.Load()
	stats.BatchesProcessed = p.batches.Load()
	stats.BackpressureCount = p.backpress.Load()
	if !p.startedAt.IsZero() {
		stats.Uptime = time.Since(p.startedAt)
	}
	return stats
}

// Watermarks returns current watermarks for all sources.
func (p *Pipeline) Watermarks() map[string]*Watermark {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*Watermark, len(p.watermarks))
	for k, v := range p.watermarks {
		cp := *v
		result[k] = &cp
	}
	return result
}

// BufferLen returns the current buffer utilization.
func (p *Pipeline) BufferLen() int {
	return len(p.buffer)
}

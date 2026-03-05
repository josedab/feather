package streamingcdc

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ManagerConfig configures the streaming CDC pipeline manager.
type ManagerConfig struct {
	MaxPipelines    int `json:"max_pipelines"`
	DefaultBuffer   int `json:"default_buffer"`
	DefaultParallel int `json:"default_parallelism"`
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxPipelines:    100,
		DefaultBuffer:   10000,
		DefaultParallel: 4,
	}
}

// PipelineInfo provides a summary of a pipeline for API responses.
type PipelineInfo struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	State     PipelineState `json:"state"`
	Source    string         `json:"source_id"`
	Target    string        `json:"target_feature_group"`
	Stats     PipelineStats `json:"stats"`
	CreatedAt time.Time     `json:"created_at"`
}

// Manager manages multiple streaming CDC pipelines.
type Manager struct {
	mu        sync.RWMutex
	config    ManagerConfig
	pipelines map[string]*Pipeline
	created   map[string]time.Time
	ctx       context.Context
}

// NewManager creates a new streaming CDC manager.
func NewManager(config ManagerConfig) *Manager {
	if config.MaxPipelines <= 0 {
		config.MaxPipelines = 100
	}
	return &Manager{
		config:    config,
		pipelines: make(map[string]*Pipeline),
		created:   make(map[string]time.Time),
		ctx:       context.Background(),
	}
}

// CreatePipeline creates and registers a new streaming pipeline.
func (m *Manager) CreatePipeline(config PipelineConfig) (*PipelineInfo, error) {
	if config.ID == "" {
		return nil, fmt.Errorf("pipeline id is required")
	}
	if config.SourceID == "" {
		return nil, fmt.Errorf("source_id is required")
	}
	if config.TargetFeatureGroup == "" {
		return nil, fmt.Errorf("target_feature_group is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pipelines[config.ID]; exists {
		return nil, fmt.Errorf("pipeline %s already exists", config.ID)
	}
	if len(m.pipelines) >= m.config.MaxPipelines {
		return nil, fmt.Errorf("maximum pipelines (%d) reached", m.config.MaxPipelines)
	}

	if config.BufferSize <= 0 {
		config.BufferSize = m.config.DefaultBuffer
	}
	if config.Parallelism <= 0 {
		config.Parallelism = m.config.DefaultParallel
	}

	pipeline := NewPipeline(config)
	m.pipelines[config.ID] = pipeline
	m.created[config.ID] = time.Now()

	return m.pipelineInfo(config.ID, pipeline), nil
}

// StartPipeline starts a pipeline by ID.
func (m *Manager) StartPipeline(id string) error {
	m.mu.RLock()
	pipeline, exists := m.pipelines[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("pipeline %s not found", id)
	}
	return pipeline.Start(m.ctx)
}

// StopPipeline stops a pipeline by ID.
func (m *Manager) StopPipeline(id string) error {
	m.mu.RLock()
	pipeline, exists := m.pipelines[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("pipeline %s not found", id)
	}
	pipeline.Stop()
	return nil
}

// DeletePipeline removes a pipeline.
func (m *Manager) DeletePipeline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipeline, exists := m.pipelines[id]
	if !exists {
		return fmt.Errorf("pipeline %s not found", id)
	}
	pipeline.Stop()
	delete(m.pipelines, id)
	delete(m.created, id)
	return nil
}

// GetPipeline returns info about a specific pipeline.
func (m *Manager) GetPipeline(id string) (*PipelineInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pipeline, exists := m.pipelines[id]
	if !exists {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}
	return m.pipelineInfo(id, pipeline), nil
}

// ListPipelines returns all pipeline summaries.
func (m *Manager) ListPipelines() []PipelineInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PipelineInfo, 0, len(m.pipelines))
	for id, p := range m.pipelines {
		result = append(result, *m.pipelineInfo(id, p))
	}
	return result
}

// IngestRecord routes a change record to the appropriate pipeline.
func (m *Manager) IngestRecord(pipelineID string, record ChangeRecord) error {
	m.mu.RLock()
	pipeline, exists := m.pipelines[pipelineID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("pipeline %s not found", pipelineID)
	}
	return pipeline.Ingest(record)
}

// IngestBatch routes multiple records to a pipeline.
func (m *Manager) IngestBatch(pipelineID string, records []ChangeRecord) (int, int, error) {
	m.mu.RLock()
	pipeline, exists := m.pipelines[pipelineID]
	m.mu.RUnlock()

	if !exists {
		return 0, 0, fmt.Errorf("pipeline %s not found", pipelineID)
	}
	ingested, dropped := pipeline.IngestBatch(records)
	return ingested, dropped, nil
}

// GetWatermarks returns watermarks for a pipeline.
func (m *Manager) GetWatermarks(pipelineID string) (map[string]*Watermark, error) {
	m.mu.RLock()
	pipeline, exists := m.pipelines[pipelineID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("pipeline %s not found", pipelineID)
	}
	return pipeline.Watermarks(), nil
}

// Stats returns aggregate manager statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	running := 0
	totalIngested := int64(0)
	totalProcessed := int64(0)

	for _, p := range m.pipelines {
		if p.State() == StateRunning {
			running++
		}
		stats := p.Stats()
		totalIngested += stats.RecordsIngested
		totalProcessed += stats.RecordsProcessed
	}

	return map[string]interface{}{
		"total_pipelines": len(m.pipelines),
		"running":         running,
		"total_ingested":  totalIngested,
		"total_processed": totalProcessed,
	}
}

func (m *Manager) pipelineInfo(id string, p *Pipeline) *PipelineInfo {
	cfg := p.Config()
	return &PipelineInfo{
		ID:        id,
		Name:      cfg.Name,
		State:     p.State(),
		Source:    cfg.SourceID,
		Target:    cfg.TargetFeatureGroup,
		Stats:     p.Stats(),
		CreatedAt: m.created[id],
	}
}

package multimodal

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sync"
	"time"
)

// EmbeddingModel identifies the embedding model.
type EmbeddingModel string

const (
	ModelCLIP     EmbeddingModel = "clip"
	ModelWhisper  EmbeddingModel = "whisper"
	ModelSentence EmbeddingModel = "sentence-transformers"
	ModelCustom   EmbeddingModel = "custom"
)

// PipelineConfig configures the embedding generation pipeline.
type PipelineConfig struct {
	DefaultModel     EmbeddingModel `json:"default_model" yaml:"default_model"`
	DefaultDimension int            `json:"default_dimension" yaml:"default_dimension"`
	BatchSize        int            `json:"batch_size" yaml:"batch_size"`
	MaxConcurrent    int            `json:"max_concurrent" yaml:"max_concurrent"`
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		DefaultModel:     ModelSentence,
		DefaultDimension: 384,
		BatchSize:        32,
		MaxConcurrent:    4,
	}
}

// PipelineItem represents one item flowing through the pipeline.
type PipelineItem struct {
	ID           string            `json:"id"`
	Modality     ModalityType      `json:"modality"`
	Data         []byte            `json:"-"`
	Embedding    []float64         `json:"embedding,omitempty"`
	Model        EmbeddingModel    `json:"model"`
	Preprocessed *PreprocessResult `json:"preprocessed,omitempty"`
	Status       string            `json:"status"` // "pending", "preprocessed", "embedded", "stored", "failed"
	Error        string            `json:"error,omitempty"`
	IngestedAt   time.Time         `json:"ingested_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
}

// PipelineStats tracks pipeline performance.
type PipelineStats struct {
	TotalIngested     int64   `json:"total_ingested"`
	TotalPreprocessed int64   `json:"total_preprocessed"`
	TotalEmbedded     int64   `json:"total_embedded"`
	TotalStored       int64   `json:"total_stored"`
	TotalFailed       int64   `json:"total_failed"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
}

// EmbeddingPipeline implements the ingest→preprocess→embed→store pipeline.
type EmbeddingPipeline struct {
	mu      sync.RWMutex
	config  PipelineConfig
	store   *MultiModalStore
	index   *EmbeddingIndex
	imgProc *ImagePreprocessor
	audProc *AudioPreprocessor
	txtProc *TextPreprocessor
	items   map[string]*PipelineItem
	stats   PipelineStats
	totalMs float64
}

// NewEmbeddingPipeline creates a new embedding generation pipeline.
func NewEmbeddingPipeline(config PipelineConfig, store *MultiModalStore, index *EmbeddingIndex) *EmbeddingPipeline {
	return &EmbeddingPipeline{
		config:  config,
		store:   store,
		index:   index,
		imgProc: NewImagePreprocessor(DefaultImageConfig()),
		audProc: NewAudioPreprocessor(DefaultAudioConfig()),
		txtProc: NewTextPreprocessor(DefaultTextConfig()),
		items:   make(map[string]*PipelineItem),
	}
}

// Ingest adds data to the pipeline for processing.
func (p *EmbeddingPipeline) Ingest(id string, modality ModalityType, data []byte, model EmbeddingModel) (*PipelineItem, error) {
	if id == "" {
		hash := sha256.Sum256(data)
		id = fmt.Sprintf("%s-%x", modality, hash[:8])
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required")
	}
	if model == "" {
		model = p.config.DefaultModel
	}

	item := &PipelineItem{
		ID:         id,
		Modality:   modality,
		Data:       data,
		Model:      model,
		Status:     "pending",
		IngestedAt: time.Now(),
	}

	p.mu.Lock()
	p.items[id] = item
	p.stats.TotalIngested++
	p.mu.Unlock()

	return item, nil
}

// Process runs the full pipeline for an item: preprocess→embed→store.
func (p *EmbeddingPipeline) Process(id string) (*PipelineItem, error) {
	p.mu.RLock()
	item, exists := p.items[id]
	p.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("item %s not found", id)
	}

	start := time.Now()

	// Step 1: Preprocess
	var result *PreprocessResult
	var err error
	switch item.Modality {
	case ModalityImage:
		result, err = p.imgProc.Process(item.Data)
	case ModalityAudio:
		result, err = p.audProc.Process(item.Data)
	case ModalityText:
		result, err = p.txtProc.Process(item.Data)
	default:
		result = &PreprocessResult{
			ID:        id,
			InputType: PreprocessorType(item.Modality),
			InputSize: int64(len(item.Data)),
		}
	}

	if err != nil {
		p.mu.Lock()
		item.Status = "failed"
		item.Error = err.Error()
		p.stats.TotalFailed++
		p.mu.Unlock()
		return item, fmt.Errorf("preprocessing: %w", err)
	}

	p.mu.Lock()
	item.Preprocessed = result
	item.Status = "preprocessed"
	p.stats.TotalPreprocessed++
	p.mu.Unlock()

	// Step 2: Generate embedding (deterministic hash-based for built-in)
	embedding := p.generateEmbedding(item.Data, p.config.DefaultDimension)
	p.mu.Lock()
	item.Embedding = embedding
	item.Status = "embedded"
	p.stats.TotalEmbedded++
	p.mu.Unlock()

	// Step 3: Store in blob store and vector index
	if p.store != nil {
		contentType := "application/octet-stream"
		switch item.Modality {
		case ModalityImage:
			contentType = "image/png"
		case ModalityAudio:
			contentType = "audio/wav"
		case ModalityText:
			contentType = "text/plain"
		}
		_, storeErr := p.store.Store(item.Modality, contentType, item.Data, nil)
		if storeErr != nil {
			p.mu.Lock()
			item.Status = "failed"
			item.Error = storeErr.Error()
			p.stats.TotalFailed++
			p.mu.Unlock()
			return item, fmt.Errorf("storing blob: %w", storeErr)
		}
	}

	if p.index != nil {
		_ = p.index.Add(id, embedding, string(item.Model))
	}

	p.mu.Lock()
	item.Status = "stored"
	now := time.Now()
	item.CompletedAt = &now
	p.stats.TotalStored++
	p.totalMs += float64(time.Since(start).Milliseconds())
	if p.stats.TotalStored > 0 {
		p.stats.AvgLatencyMs = p.totalMs / float64(p.stats.TotalStored)
	}
	p.mu.Unlock()

	return item, nil
}

// IngestAndProcess is a convenience method that ingests and processes in one call.
func (p *EmbeddingPipeline) IngestAndProcess(id string, modality ModalityType, data []byte, model EmbeddingModel) (*PipelineItem, error) {
	item, err := p.Ingest(id, modality, data, model)
	if err != nil {
		return nil, err
	}
	return p.Process(item.ID)
}

// GetItem returns a pipeline item by ID.
func (p *EmbeddingPipeline) GetItem(id string) (*PipelineItem, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	item, exists := p.items[id]
	if !exists {
		return nil, fmt.Errorf("item %s not found", id)
	}
	cp := *item
	return &cp, nil
}

// Stats returns pipeline statistics.
func (p *EmbeddingPipeline) Stats() PipelineStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// BatchItem describes one item in a batch processing request.
type BatchItem struct {
	ID       string       `json:"id"`
	Modality ModalityType `json:"modality"`
	Data     []byte       `json:"-"`
	Model    EmbeddingModel `json:"model,omitempty"`
}

// BatchResult summarizes the outcome of a batch operation.
type BatchResult struct {
	Total     int            `json:"total"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Items     []*PipelineItem `json:"items"`
	Errors    []string       `json:"errors,omitempty"`
}

// ProcessBatch ingests and processes multiple items sequentially.
func (p *EmbeddingPipeline) ProcessBatch(items []BatchItem) *BatchResult {
	result := &BatchResult{Total: len(items)}
	for _, bi := range items {
		item, err := p.IngestAndProcess(bi.ID, bi.Modality, bi.Data, bi.Model)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", bi.ID, err))
			continue
		}
		result.Succeeded++
		result.Items = append(result.Items, item)
	}
	return result
}

// generateEmbedding creates a deterministic embedding from data.
// In production, this would call an external model (CLIP, Whisper, etc.).
func (p *EmbeddingPipeline) generateEmbedding(data []byte, dims int) []float64 {
	hash := sha256.Sum256(data)
	embedding := make([]float64, dims)
	for i := 0; i < dims; i++ {
		byteIdx := i % len(hash)
		embedding[i] = float64(hash[byteIdx])/255.0*2.0 - 1.0
	}
	// L2 normalize
	var norm float64
	for _, v := range embedding {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	return embedding
}

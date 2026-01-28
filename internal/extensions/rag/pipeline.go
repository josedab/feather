package rag

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Pipeline errors.
var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrEmptyContent     = errors.New("document content is empty")
	ErrMaxDocuments     = errors.New("maximum document limit reached")
	ErrNilDocument      = errors.New("document is nil")
)

// PipelineConfig configures the RAG pipeline.
type PipelineConfig struct {
	// ChunkSize is the target chunk size in characters.
	ChunkSize int `json:"chunk_size"`
	// ChunkOverlap is the overlap between consecutive chunks.
	ChunkOverlap int `json:"chunk_overlap"`
	// MaxDocuments is the maximum number of documents to store.
	MaxDocuments int `json:"max_documents"`
	// EmbeddingDim is the embedding vector dimensionality.
	EmbeddingDim int `json:"embedding_dim"`
	// SimilarityMetric is the similarity function (cosine, euclidean, dot_product).
	SimilarityMetric string `json:"similarity_metric"`
	// TopK is the default number of results to return.
	TopK int `json:"top_k"`
	// MinScore is the minimum similarity score threshold.
	MinScore float64 `json:"min_score"`
	// IndexName is the name of the vector index.
	IndexName string `json:"index_name"`
	// BatchSize is the max batch size for embedding requests.
	BatchSize int `json:"batch_size"`
	// EnableCache enables embedding caching.
	EnableCache bool `json:"enable_cache"`
}

// DefaultPipelineConfig returns sensible defaults for the RAG pipeline.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		ChunkSize:        512,
		ChunkOverlap:     50,
		MaxDocuments:     10000,
		EmbeddingDim:     128,
		SimilarityMetric: "cosine",
		TopK:             5,
		MinScore:         0.0,
		IndexName:        "rag_default",
		BatchSize:        32,
		EnableCache:      true,
	}
}

// Document represents a document ingested into the RAG pipeline.
type Document struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Source    string            `json:"source,omitempty"`
	Chunks    []*Chunk          `json:"chunks,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Chunk represents a piece of a document with position and embedding info.
type Chunk struct {
	ID         string            `json:"id"`
	DocumentID string            `json:"document_id"`
	Content    string            `json:"content"`
	Index      int               `json:"index"`
	StartPos   int               `json:"start_pos"`
	EndPos     int               `json:"end_pos"`
	Embedding  []float32         `json:"embedding,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// RetrievalResult holds the results of a retrieval query.
type RetrievalResult struct {
	Chunks    []*ScoredChunk `json:"chunks"`
	Query     string         `json:"query"`
	TotalHits int            `json:"total_hits"`
	Duration  time.Duration  `json:"duration"`
}

// ScoredChunk pairs a chunk with its similarity score.
type ScoredChunk struct {
	Chunk *Chunk  `json:"chunk"`
	Score float64 `json:"score"`
}

// PipelineStats contains pipeline statistics.
type PipelineStats struct {
	DocumentCount int    `json:"document_count"`
	ChunkCount    int    `json:"chunk_count"`
	IndexSize     int    `json:"index_size"`
	IndexName     string `json:"index_name"`
	EmbeddingDim  int    `json:"embedding_dim"`
}

// Pipeline orchestrates document ingestion, chunking, embedding, and retrieval.
type Pipeline struct {
	config    PipelineConfig
	chunker   *Chunker
	indexer   *Indexer
	retriever *Retriever
	embedder  Embedder
	documents map[string]*Document
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPipeline creates a new RAG pipeline with the given configuration.
func NewPipeline(config PipelineConfig) *Pipeline {
	if config.ChunkSize <= 0 {
		config.ChunkSize = 512
	}
	if config.EmbeddingDim <= 0 {
		config.EmbeddingDim = 128
	}
	if config.MaxDocuments <= 0 {
		config.MaxDocuments = 10000
	}
	if config.TopK <= 0 {
		config.TopK = 5
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 32
	}
	if config.SimilarityMetric == "" {
		config.SimilarityMetric = "cosine"
	}

	ctx, cancel := context.WithCancel(context.Background())

	embedder := NewLocalEmbedder(config.EmbeddingDim)
	indexer := NewIndexer(config.EmbeddingDim, config.SimilarityMetric)
	retriever := NewRetriever(indexer, config.TopK, config.MinScore)

	return &Pipeline{
		config:    config,
		chunker:   NewChunker(ChunkBySize, config.ChunkSize, config.ChunkOverlap),
		indexer:   indexer,
		retriever: retriever,
		embedder:  embedder,
		documents: make(map[string]*Document),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Ingest processes a single document: assigns an ID, chunks, embeds, and indexes it.
func (p *Pipeline) Ingest(ctx context.Context, doc *Document) error {
	if doc == nil {
		return ErrNilDocument
	}
	if len(doc.Content) == 0 {
		return ErrEmptyContent
	}

	p.mu.Lock()
	if p.config.MaxDocuments > 0 && len(p.documents) >= p.config.MaxDocuments {
		p.mu.Unlock()
		return ErrMaxDocuments
	}
	p.mu.Unlock()

	now := time.Now()
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	doc.CreatedAt = now
	doc.UpdatedAt = now

	// Chunk
	chunks := p.chunker.Chunk(doc.Content)
	for _, ch := range chunks {
		ch.DocumentID = doc.ID
		if ch.Metadata == nil {
			ch.Metadata = make(map[string]string)
		}
		// Propagate document metadata to chunks.
		for k, v := range doc.Metadata {
			ch.Metadata[k] = v
		}
	}

	// Embed
	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch.Content
	}

	embeddings, err := p.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding chunks: %w", err)
	}

	// Index
	for i, ch := range chunks {
		ch.Embedding = embeddings[i]
		if err := p.indexer.Add(ch.ID, embeddings[i]); err != nil {
			return fmt.Errorf("indexing chunk %s: %w", ch.ID, err)
		}
	}

	doc.Chunks = chunks

	p.mu.Lock()
	p.documents[doc.ID] = doc
	p.mu.Unlock()

	return nil
}

// IngestBatch ingests multiple documents.
func (p *Pipeline) IngestBatch(ctx context.Context, docs []*Document) error {
	for _, doc := range docs {
		if err := p.Ingest(ctx, doc); err != nil {
			return fmt.Errorf("ingesting document %s: %w", doc.ID, err)
		}
	}
	return nil
}

// Delete removes a document and all its chunks from the pipeline.
func (p *Pipeline) Delete(ctx context.Context, docID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	doc, ok := p.documents[docID]
	if !ok {
		return fmt.Errorf("deleting document %s: %w", docID, ErrDocumentNotFound)
	}

	for _, ch := range doc.Chunks {
		if err := p.indexer.Delete(ch.ID); err != nil {
			return fmt.Errorf("deleting chunk %s: %w", ch.ID, err)
		}
	}

	delete(p.documents, docID)
	return nil
}

// Retrieve performs semantic search and returns the top-K most relevant chunks.
func (p *Pipeline) Retrieve(ctx context.Context, query string, topK int) (*RetrievalResult, error) {
	start := time.Now()

	if topK <= 0 {
		topK = p.config.TopK
	}

	queryEmb, err := p.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	scored := p.retriever.Search(queryEmb, topK)

	// Resolve chunk objects from the scored results.
	result := &RetrievalResult{
		Query:     query,
		Chunks:    make([]*ScoredChunk, 0, len(scored)),
		TotalHits: len(scored),
		Duration:  time.Since(start),
	}

	p.mu.RLock()
	chunkMap := p.buildChunkMap()
	p.mu.RUnlock()

	for _, sr := range scored {
		if ch, ok := chunkMap[sr.ID]; ok {
			result.Chunks = append(result.Chunks, &ScoredChunk{
				Chunk: ch,
				Score: sr.Score,
			})
		}
	}
	result.TotalHits = len(result.Chunks)

	return result, nil
}

// RetrieveWithFilter performs semantic search with metadata filtering.
func (p *Pipeline) RetrieveWithFilter(ctx context.Context, query string, topK int, filters map[string]string) (*RetrievalResult, error) {
	start := time.Now()

	if topK <= 0 {
		topK = p.config.TopK
	}

	queryEmb, err := p.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	// Search with a larger candidate set to allow for filtering.
	candidateK := topK * 5
	if candidateK < 50 {
		candidateK = 50
	}
	scored := p.retriever.Search(queryEmb, candidateK)

	p.mu.RLock()
	chunkMap := p.buildChunkMap()
	p.mu.RUnlock()

	result := &RetrievalResult{
		Query:    query,
		Chunks:   make([]*ScoredChunk, 0, topK),
		Duration: time.Since(start),
	}

	for _, sr := range scored {
		if len(result.Chunks) >= topK {
			break
		}
		ch, ok := chunkMap[sr.ID]
		if !ok {
			continue
		}
		if matchesFilters(ch.Metadata, filters) {
			result.Chunks = append(result.Chunks, &ScoredChunk{
				Chunk: ch,
				Score: sr.Score,
			})
		}
	}
	result.TotalHits = len(result.Chunks)

	return result, nil
}

// GetDocument returns a document by ID.
func (p *Pipeline) GetDocument(_ context.Context, id string) (*Document, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	doc, ok := p.documents[id]
	if !ok {
		return nil, fmt.Errorf("getting document %s: %w", id, ErrDocumentNotFound)
	}
	return doc, nil
}

// ListDocuments returns all stored documents.
func (p *Pipeline) ListDocuments(_ context.Context) []*Document {
	p.mu.RLock()
	defer p.mu.RUnlock()

	docs := make([]*Document, 0, len(p.documents))
	for _, doc := range p.documents {
		docs = append(docs, doc)
	}
	return docs
}

// UpdateDocument replaces a document's content, re-chunking and re-indexing.
func (p *Pipeline) UpdateDocument(ctx context.Context, doc *Document) error {
	if doc == nil {
		return ErrNilDocument
	}

	p.mu.RLock()
	existing, ok := p.documents[doc.ID]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("updating document %s: %w", doc.ID, ErrDocumentNotFound)
	}

	// Remove old chunks from index.
	for _, ch := range existing.Chunks {
		if err := p.indexer.Delete(ch.ID); err != nil {
			return fmt.Errorf("removing old chunk %s: %w", ch.ID, err)
		}
	}

	// Re-chunk and embed.
	doc.CreatedAt = existing.CreatedAt
	doc.UpdatedAt = time.Now()

	chunks := p.chunker.Chunk(doc.Content)
	for _, ch := range chunks {
		ch.DocumentID = doc.ID
		if ch.Metadata == nil {
			ch.Metadata = make(map[string]string)
		}
		for k, v := range doc.Metadata {
			ch.Metadata[k] = v
		}
	}

	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch.Content
	}

	embeddings, err := p.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding updated chunks: %w", err)
	}

	for i, ch := range chunks {
		ch.Embedding = embeddings[i]
		if err := p.indexer.Add(ch.ID, embeddings[i]); err != nil {
			return fmt.Errorf("indexing updated chunk %s: %w", ch.ID, err)
		}
	}

	doc.Chunks = chunks

	p.mu.Lock()
	p.documents[doc.ID] = doc
	p.mu.Unlock()

	return nil
}

// Stats returns current pipeline statistics.
func (p *Pipeline) Stats() *PipelineStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	chunkCount := 0
	for _, doc := range p.documents {
		chunkCount += len(doc.Chunks)
	}

	return &PipelineStats{
		DocumentCount: len(p.documents),
		ChunkCount:    chunkCount,
		IndexSize:     p.indexer.Count(),
		IndexName:     p.config.IndexName,
		EmbeddingDim:  p.config.EmbeddingDim,
	}
}

// Close shuts down the pipeline and releases resources.
func (p *Pipeline) Close() error {
	p.cancel()
	return nil
}

// buildChunkMap creates a map from chunk ID to Chunk across all documents.
// Must be called with at least a read lock held.
func (p *Pipeline) buildChunkMap() map[string]*Chunk {
	m := make(map[string]*Chunk)
	for _, doc := range p.documents {
		for _, ch := range doc.Chunks {
			m[ch.ID] = ch
		}
	}
	return m
}

// matchesFilters returns true if the metadata contains all key-value pairs from filters.
func matchesFilters(metadata, filters map[string]string) bool {
	for k, v := range filters {
		if mv, ok := metadata[k]; !ok || mv != v {
			return false
		}
	}
	return true
}

// Retriever wraps the indexer with score filtering.
type Retriever struct {
	indexer  *Indexer
	topK     int
	minScore float64
}

// NewRetriever creates a Retriever that delegates to the indexer.
func NewRetriever(indexer *Indexer, topK int, minScore float64) *Retriever {
	return &Retriever{
		indexer:  indexer,
		topK:     topK,
		minScore: minScore,
	}
}

// Search returns top-K results above the minimum score threshold.
func (r *Retriever) Search(query []float32, topK int) []*SearchResult {
	if topK <= 0 {
		topK = r.topK
	}

	results := r.indexer.Search(query, topK)

	// Filter by minimum score.
	if r.minScore > 0 {
		filtered := make([]*SearchResult, 0, len(results))
		for _, sr := range results {
			if sr.Score >= r.minScore {
				filtered = append(filtered, sr)
			}
		}
		return filtered
	}
	return results
}

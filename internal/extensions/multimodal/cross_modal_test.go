package multimodal

import (
	"math"
	"testing"
)

func TestCrossModalSearch_SearchByVector(t *testing.T) {
	config := DefaultPipelineConfig()
	indexConfig := EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	}
	index := NewEmbeddingIndex(indexConfig)
	pipeline := NewEmbeddingPipeline(config, nil, index)

	// Add items via pipeline
	_, err := pipeline.IngestAndProcess("item-1", ModalityText, []byte("hello world"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	_, err = pipeline.IngestAndProcess("item-2", ModalityImage, []byte("image data"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}

	search := NewCrossModalSearch(DefaultCrossModalSearchConfig(), index, nil, pipeline)

	query := pipeline.generateEmbedding([]byte("hello world"), config.DefaultDimension)
	results, err := search.SearchByVector(query, 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	// First result should be the exact match
	if results[0].BlobID != "item-1" {
		t.Errorf("expected first result 'item-1', got %s", results[0].BlobID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("expected near-perfect score for exact match, got %f", results[0].Score)
	}
}

func TestCrossModalSearch_SearchByText(t *testing.T) {
	config := DefaultPipelineConfig()
	indexConfig := EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	}
	index := NewEmbeddingIndex(indexConfig)
	pipeline := NewEmbeddingPipeline(config, nil, index)

	_, err := pipeline.IngestAndProcess("txt-1", ModalityText, []byte("search target text"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}

	search := NewCrossModalSearch(DefaultCrossModalSearchConfig(), index, nil, pipeline)

	results, err := search.SearchByText("search target text", 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}

func TestCrossModalSearch_SearchByBlob(t *testing.T) {
	config := DefaultPipelineConfig()
	indexConfig := EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	}
	index := NewEmbeddingIndex(indexConfig)
	pipeline := NewEmbeddingPipeline(config, nil, index)

	_, err := pipeline.IngestAndProcess("blob-1", ModalityText, []byte("first item"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	_, err = pipeline.IngestAndProcess("blob-2", ModalityText, []byte("second item"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	_, err = pipeline.IngestAndProcess("blob-3", ModalityImage, []byte("third item"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}

	search := NewCrossModalSearch(DefaultCrossModalSearchConfig(), index, nil, pipeline)

	results, err := search.SearchByBlob("blob-1", 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	// Self should be excluded
	for _, r := range results {
		if r.BlobID == "blob-1" {
			t.Error("self-match should be excluded from results")
		}
	}
}

func TestCrossModalSearch_SearchByBlobNotFound(t *testing.T) {
	indexConfig := EmbeddingConfig{
		DefaultDimensions: 384,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	}
	index := NewEmbeddingIndex(indexConfig)
	search := NewCrossModalSearch(DefaultCrossModalSearchConfig(), index, nil, nil)

	_, err := search.SearchByBlob("nonexistent", 5)
	if err == nil {
		t.Fatal("expected error for nonexistent blob")
	}
}

func TestCrossModalSearch_NilIndex(t *testing.T) {
	search := NewCrossModalSearch(DefaultCrossModalSearchConfig(), nil, nil, nil)

	_, err := search.SearchByVector([]float64{1, 2, 3}, 5)
	if err == nil {
		t.Fatal("expected error for nil index")
	}
}

func TestCrossModalSearch_NilPipeline(t *testing.T) {
	search := NewCrossModalSearch(DefaultCrossModalSearchConfig(), nil, nil, nil)

	_, err := search.SearchByText("test", 5)
	if err == nil {
		t.Fatal("expected error for nil pipeline")
	}
}

func TestCrossModalSearch_WithStore(t *testing.T) {
	config := DefaultPipelineConfig()
	storeConfig := DefaultStoreConfig()
	store := NewMultiModalStore(storeConfig)
	indexConfig := EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	}
	index := NewEmbeddingIndex(indexConfig)
	pipeline := NewEmbeddingPipeline(config, store, index)

	_, err := pipeline.IngestAndProcess("ws-1", ModalityText, []byte("with store test"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}

	search := NewCrossModalSearch(DefaultCrossModalSearchConfig(), index, store, pipeline)

	query := pipeline.generateEmbedding([]byte("with store test"), config.DefaultDimension)
	results, err := search.SearchByVector(query, 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestCrossModal_CosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "different lengths",
			a:        []float64{1, 0},
			b:        []float64{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "zero vector",
			a:        []float64{0, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.expected) > 1e-10 {
				t.Errorf("CosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestCrossModalSearch_MinScore(t *testing.T) {
	config := DefaultPipelineConfig()
	indexConfig := EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	}
	index := NewEmbeddingIndex(indexConfig)
	pipeline := NewEmbeddingPipeline(config, nil, index)

	_, err := pipeline.IngestAndProcess("ms-1", ModalityText, []byte("alpha"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	_, err = pipeline.IngestAndProcess("ms-2", ModalityText, []byte("beta"), "")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}

	searchConfig := CrossModalSearchConfig{
		MaxResults: 10,
		MinScore:   0.999, // Very high threshold
	}
	search := NewCrossModalSearch(searchConfig, index, nil, pipeline)

	query := pipeline.generateEmbedding([]byte("alpha"), config.DefaultDimension)
	results, err := search.SearchByVector(query, 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	// Only the exact match should pass the threshold
	for _, r := range results {
		if r.Score < 0.999 {
			t.Errorf("result %s has score %f below min threshold", r.BlobID, r.Score)
		}
	}
}

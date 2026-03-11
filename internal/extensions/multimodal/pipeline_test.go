package multimodal

import (
	"testing"
)

func TestEmbeddingPipeline_Ingest(t *testing.T) {
	pipeline := NewEmbeddingPipeline(DefaultPipelineConfig(), nil, nil)

	item, err := pipeline.Ingest("test-1", ModalityText, []byte("hello world"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID != "test-1" {
		t.Errorf("expected ID 'test-1', got %s", item.ID)
	}
	if item.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", item.Status)
	}
	if item.Model != ModelSentence {
		t.Errorf("expected default model %s, got %s", ModelSentence, item.Model)
	}
}

func TestEmbeddingPipeline_IngestEmptyData(t *testing.T) {
	pipeline := NewEmbeddingPipeline(DefaultPipelineConfig(), nil, nil)

	_, err := pipeline.Ingest("test-1", ModalityText, []byte{}, "")
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestEmbeddingPipeline_IngestAutoID(t *testing.T) {
	pipeline := NewEmbeddingPipeline(DefaultPipelineConfig(), nil, nil)

	item, err := pipeline.Ingest("", ModalityImage, []byte("image data"), ModelCLIP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if item.Model != ModelCLIP {
		t.Errorf("expected model %s, got %s", ModelCLIP, item.Model)
	}
}

func TestEmbeddingPipeline_ProcessText(t *testing.T) {
	config := DefaultPipelineConfig()
	index := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	})
	pipeline := NewEmbeddingPipeline(config, nil, index)

	_, err := pipeline.Ingest("txt-1", ModalityText, []byte("hello world test"), "")
	if err != nil {
		t.Fatalf("ingest error: %v", err)
	}

	item, err := pipeline.Process("txt-1")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}

	if item.Status != "stored" {
		t.Errorf("expected status 'stored', got %s", item.Status)
	}
	if item.Preprocessed == nil {
		t.Error("expected preprocessed result")
	}
	if len(item.Embedding) != config.DefaultDimension {
		t.Errorf("expected embedding dim %d, got %d", config.DefaultDimension, len(item.Embedding))
	}
	if item.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestEmbeddingPipeline_ProcessImage(t *testing.T) {
	config := DefaultPipelineConfig()
	index := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	})
	pipeline := NewEmbeddingPipeline(config, nil, index)

	_, err := pipeline.Ingest("img-1", ModalityImage, []byte("fake image bytes"), ModelCLIP)
	if err != nil {
		t.Fatalf("ingest error: %v", err)
	}

	item, err := pipeline.Process("img-1")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	if item.Status != "stored" {
		t.Errorf("expected status 'stored', got %s", item.Status)
	}
}

func TestEmbeddingPipeline_ProcessAudio(t *testing.T) {
	config := DefaultPipelineConfig()
	index := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	})
	pipeline := NewEmbeddingPipeline(config, nil, index)

	audioData := make([]byte, 32000)
	for i := range audioData {
		audioData[i] = byte(i % 256)
	}

	_, err := pipeline.Ingest("aud-1", ModalityAudio, audioData, ModelWhisper)
	if err != nil {
		t.Fatalf("ingest error: %v", err)
	}

	item, err := pipeline.Process("aud-1")
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	if item.Status != "stored" {
		t.Errorf("expected status 'stored', got %s", item.Status)
	}
}

func TestEmbeddingPipeline_IngestAndProcess(t *testing.T) {
	config := DefaultPipelineConfig()
	index := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	})
	pipeline := NewEmbeddingPipeline(config, nil, index)

	item, err := pipeline.IngestAndProcess("conv-1", ModalityText, []byte("convenience method test"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Status != "stored" {
		t.Errorf("expected status 'stored', got %s", item.Status)
	}
}

func TestEmbeddingPipeline_ProcessNotFound(t *testing.T) {
	pipeline := NewEmbeddingPipeline(DefaultPipelineConfig(), nil, nil)

	_, err := pipeline.Process("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
}

func TestEmbeddingPipeline_GetItem(t *testing.T) {
	pipeline := NewEmbeddingPipeline(DefaultPipelineConfig(), nil, nil)

	_, err := pipeline.Ingest("get-1", ModalityText, []byte("test"), "")
	if err != nil {
		t.Fatalf("ingest error: %v", err)
	}

	item, err := pipeline.GetItem("get-1")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if item.ID != "get-1" {
		t.Errorf("expected ID 'get-1', got %s", item.ID)
	}
}

func TestEmbeddingPipeline_GetItemNotFound(t *testing.T) {
	pipeline := NewEmbeddingPipeline(DefaultPipelineConfig(), nil, nil)

	_, err := pipeline.GetItem("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
}

func TestEmbeddingPipeline_Stats(t *testing.T) {
	config := DefaultPipelineConfig()
	index := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	})
	pipeline := NewEmbeddingPipeline(config, nil, index)

	for i := 0; i < 3; i++ {
		data := []byte("test data " + string(rune('a'+i)))
		_, err := pipeline.IngestAndProcess("", ModalityText, data, "")
		if err != nil {
			t.Fatalf("process error on item %d: %v", i, err)
		}
	}

	stats := pipeline.Stats()
	if stats.TotalIngested != 3 {
		t.Errorf("expected 3 ingested, got %d", stats.TotalIngested)
	}
	if stats.TotalStored != 3 {
		t.Errorf("expected 3 stored, got %d", stats.TotalStored)
	}
	if stats.TotalFailed != 0 {
		t.Errorf("expected 0 failed, got %d", stats.TotalFailed)
	}
}

func TestEmbeddingPipeline_WithStore(t *testing.T) {
	config := DefaultPipelineConfig()
	storeConfig := DefaultStoreConfig()
	store := NewMultiModalStore(storeConfig)
	index := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: config.DefaultDimension,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     1000,
	})
	pipeline := NewEmbeddingPipeline(config, store, index)

	item, err := pipeline.IngestAndProcess("store-1", ModalityText, []byte("stored text"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Status != "stored" {
		t.Errorf("expected status 'stored', got %s", item.Status)
	}
}

func TestEmbeddingPipeline_GenerateEmbedding(t *testing.T) {
	pipeline := NewEmbeddingPipeline(DefaultPipelineConfig(), nil, nil)

	data := []byte("test data")
	emb1 := pipeline.generateEmbedding(data, 128)
	emb2 := pipeline.generateEmbedding(data, 128)

	if len(emb1) != 128 {
		t.Errorf("expected dimension 128, got %d", len(emb1))
	}

	// Deterministic: same input → same output
	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Errorf("embedding not deterministic at index %d", i)
			break
		}
	}

	// L2 normalized
	var norm float64
	for _, v := range emb1 {
		norm += v * v
	}
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("expected unit norm, got %f", norm)
	}
}

package multimodal

import (
	"math"
	"testing"
)

func TestEmbeddingAddAndGet(t *testing.T) {
	idx := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: 3,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     100,
	})

	vec := []float64{1.0, 0.0, 0.0}
	if err := idx.Add("blob_1", vec, "test-model"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entry, err := idx.Get("blob_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Model != "test-model" {
		t.Errorf("model = %q, want test-model", entry.Model)
	}
	if len(entry.Vector) != 3 {
		t.Errorf("vector length = %d, want 3", len(entry.Vector))
	}
}

func TestEmbeddingNotFound(t *testing.T) {
	idx := NewEmbeddingIndex(DefaultEmbeddingConfig())
	_, err := idx.Get("nonexistent")
	if err != ErrEmbeddingNotFound {
		t.Errorf("err = %v, want ErrEmbeddingNotFound", err)
	}
}

func TestEmbeddingDimensionMismatch(t *testing.T) {
	idx := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: 3,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     100,
	})

	err := idx.Add("blob_1", []float64{1.0, 2.0}, "model")
	if err != ErrDimensionMismatch {
		t.Errorf("err = %v, want ErrDimensionMismatch", err)
	}
}

func TestEmbeddingRemove(t *testing.T) {
	idx := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: 2,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     100,
	})

	idx.Add("blob_1", []float64{1.0, 0.0}, "model")
	if err := idx.Remove("blob_1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err := idx.Get("blob_1")
	if err != ErrEmbeddingNotFound {
		t.Errorf("expected ErrEmbeddingNotFound after remove")
	}
}

func TestEmbeddingSearch(t *testing.T) {
	idx := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: 3,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     100,
	})

	idx.Add("blob_x", []float64{1.0, 0.0, 0.0}, "model")
	idx.Add("blob_y", []float64{0.0, 1.0, 0.0}, "model")
	idx.Add("blob_z", []float64{0.9, 0.1, 0.0}, "model")

	results := idx.Search([]float64{1.0, 0.0, 0.0}, 2)
	if len(results) != 2 {
		t.Fatalf("search returned %d results, want 2", len(results))
	}

	// blob_x should be the best match (cosine=1.0)
	if results[0].BlobID != "blob_x" {
		t.Errorf("top result = %q, want blob_x", results[0].BlobID)
	}
	if math.Abs(results[0].Score-1.0) > 1e-9 {
		t.Errorf("top score = %f, want 1.0", results[0].Score)
	}

	// blob_z should be second (closer to [1,0,0] than blob_y)
	if results[1].BlobID != "blob_z" {
		t.Errorf("second result = %q, want blob_z", results[1].BlobID)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{1, 0, 0}, []float64{1, 0, 0}, 1.0},
		{"orthogonal", []float64{1, 0, 0}, []float64{0, 1, 0}, 0.0},
		{"opposite", []float64{1, 0}, []float64{-1, 0}, -1.0},
		{"empty", nil, nil, 0.0},
		{"mismatched", []float64{1}, []float64{1, 2}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("cosineSimilarity = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestEmbeddingStats(t *testing.T) {
	idx := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: 2,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     100,
	})

	idx.Add("b1", []float64{1, 0}, "modelA")
	idx.Add("b2", []float64{0, 1}, "modelA")
	idx.Add("b3", []float64{1, 1}, "modelB")

	stats := idx.Stats()
	if stats.TotalEmbeddings != 3 {
		t.Errorf("total = %d, want 3", stats.TotalEmbeddings)
	}
	if stats.Dimensions != 2 {
		t.Errorf("dimensions = %d, want 2", stats.Dimensions)
	}
	if stats.Models["modelA"] != 2 || stats.Models["modelB"] != 1 {
		t.Errorf("unexpected models: %v", stats.Models)
	}
}

func TestMaxEmbeddings(t *testing.T) {
	idx := NewEmbeddingIndex(EmbeddingConfig{
		DefaultDimensions: 2,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     2,
	})

	idx.Add("b1", []float64{1, 0}, "m")
	idx.Add("b2", []float64{0, 1}, "m")
	err := idx.Add("b3", []float64{1, 1}, "m")
	if err != ErrMaxEmbeddingsReached {
		t.Errorf("expected ErrMaxEmbeddingsReached, got %v", err)
	}
}

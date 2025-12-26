package vector

import (
	"math"
	"math/rand"
	"testing"
)

func TestHNSW_InsertAndSearch(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          3,
		M:            16,
		EfConstruct:  200,
		DistanceType: DistanceCosine,
	})

	// Insert some vectors
	vectors := []struct {
		id     string
		vector []float32
	}{
		{"v1", []float32{1.0, 0.0, 0.0}},
		{"v2", []float32{0.0, 1.0, 0.0}},
		{"v3", []float32{0.0, 0.0, 1.0}},
		{"v4", []float32{0.707, 0.707, 0.0}},
		{"v5", []float32{0.577, 0.577, 0.577}},
	}

	for _, v := range vectors {
		if err := hnsw.Insert(v.id, v.vector); err != nil {
			t.Fatalf("Insert(%s) error: %v", v.id, err)
		}
	}

	if hnsw.Size() != 5 {
		t.Errorf("Size() = %d, want 5", hnsw.Size())
	}

	// Search for nearest to [1, 0, 0]
	query := []float32{1.0, 0.0, 0.0}
	results, err := hnsw.Search(query, 3, 0)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// v1 should be closest (identical)
	if results[0].ID != "v1" {
		t.Errorf("closest result ID = %s, want v1", results[0].ID)
	}

	// Distance should be near 0 for v1
	if results[0].Distance > 0.001 {
		t.Errorf("distance to v1 = %f, want ~0", results[0].Distance)
	}
}

func TestHNSW_DimensionMismatch(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{Dim: 3})

	// Insert with wrong dimension
	err := hnsw.Insert("v1", []float32{1.0, 0.0})
	if err != ErrDimensionMismatch {
		t.Errorf("Insert with wrong dim: got %v, want ErrDimensionMismatch", err)
	}

	// Insert with correct dimension
	if err := hnsw.Insert("v1", []float32{1.0, 0.0, 0.0}); err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	// Search with wrong dimension
	_, err = hnsw.Search([]float32{1.0, 0.0}, 1, 0)
	if err != ErrDimensionMismatch {
		t.Errorf("Search with wrong dim: got %v, want ErrDimensionMismatch", err)
	}
}

func TestHNSW_Delete(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{Dim: 3})

	hnsw.Insert("v1", []float32{1.0, 0.0, 0.0})
	hnsw.Insert("v2", []float32{0.0, 1.0, 0.0})
	hnsw.Insert("v3", []float32{0.0, 0.0, 1.0})

	if hnsw.Size() != 3 {
		t.Fatalf("Size() = %d, want 3", hnsw.Size())
	}

	if err := hnsw.Delete("v2"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if hnsw.Size() != 2 {
		t.Errorf("Size() after delete = %d, want 2", hnsw.Size())
	}

	// v2 should not be found
	_, exists := hnsw.Get("v2")
	if exists {
		t.Error("Get(v2) should return false after delete")
	}

	// Search should not return v2
	results, _ := hnsw.Search([]float32{0.0, 1.0, 0.0}, 5, 0)
	for _, r := range results {
		if r.ID == "v2" {
			t.Error("v2 should not appear in search results after delete")
		}
	}
}

func TestHNSW_Update(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{Dim: 3})

	hnsw.Insert("v1", []float32{1.0, 0.0, 0.0})

	// Update the vector
	hnsw.Insert("v1", []float32{0.0, 1.0, 0.0})

	vec, exists := hnsw.Get("v1")
	if !exists {
		t.Fatal("v1 should exist after update")
	}

	// Should have new value
	if vec[0] != 0.0 || vec[1] != 1.0 || vec[2] != 0.0 {
		t.Errorf("vector after update = %v, want [0, 1, 0]", vec)
	}
}

func TestHNSW_EuclideanDistance(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          2,
		DistanceType: DistanceEuclidean,
	})

	hnsw.Insert("origin", []float32{0.0, 0.0})
	hnsw.Insert("one_one", []float32{1.0, 1.0})
	hnsw.Insert("two_zero", []float32{2.0, 0.0})

	// Search from [0, 0]
	results, _ := hnsw.Search([]float32{0.0, 0.0}, 3, 0)

	// Origin should be closest (distance 0)
	if results[0].ID != "origin" {
		t.Errorf("closest to origin should be origin, got %s", results[0].ID)
	}

	// one_one is at distance sqrt(2) ≈ 1.414
	// two_zero is at distance 2
	if results[1].ID != "one_one" {
		t.Errorf("second closest should be one_one, got %s", results[1].ID)
	}
}

func TestHNSW_DotProductDistance(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          2,
		DistanceType: DistanceDotProduct,
	})

	// Normalized vectors
	hnsw.Insert("v1", []float32{1.0, 0.0})
	hnsw.Insert("v2", []float32{0.707, 0.707})
	hnsw.Insert("v3", []float32{0.0, 1.0})

	// Query with [1, 0] - highest dot product with v1
	results, _ := hnsw.Search([]float32{1.0, 0.0}, 3, 0)

	// v1 should be best match (highest dot product = lowest distance)
	if results[0].ID != "v1" {
		t.Errorf("best dot product match should be v1, got %s", results[0].ID)
	}
}

func TestHNSW_EmptyIndex(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{Dim: 3})

	results, err := hnsw.Search([]float32{1.0, 0.0, 0.0}, 5, 0)
	if err != nil {
		t.Fatalf("Search on empty index error: %v", err)
	}

	if results != nil && len(results) != 0 {
		t.Errorf("Search on empty index should return empty, got %d results", len(results))
	}
}

func BenchmarkHNSW_Insert(b *testing.B) {
	dim := 128
	hnsw := NewHNSW(HNSWConfig{
		Dim:         dim,
		M:           16,
		EfConstruct: 100,
	})

	vectors := make([][]float32, b.N)
	for i := range vectors {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		vectors[i] = vec
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hnsw.Insert(string(rune(i)), vectors[i])
	}
}

func BenchmarkHNSW_Search(b *testing.B) {
	dim := 128
	numVectors := 10000
	hnsw := NewHNSW(HNSWConfig{
		Dim:         dim,
		M:           16,
		EfConstruct: 100,
	})

	// Insert vectors
	for i := 0; i < numVectors; i++ {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		hnsw.Insert(string(rune(i)), vec)
	}

	// Create query
	query := make([]float32, dim)
	for i := range query {
		query[i] = rand.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hnsw.Search(query, 10, 50)
	}
}

func TestCosineDistance(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 1,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineDistance(tt.a, tt.b)
			if math.Abs(float64(got-tt.expected)) > 0.001 {
				t.Errorf("cosineDistance() = %f, want %f", got, tt.expected)
			}
		})
	}
}

func TestEuclideanDistance(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 0,
		},
		{
			name:     "unit distance",
			a:        []float32{0, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1,
		},
		{
			name:     "pythagorean",
			a:        []float32{0, 0},
			b:        []float32{3, 4},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := euclideanDistance(tt.a, tt.b)
			if math.Abs(float64(got-tt.expected)) > 0.001 {
				t.Errorf("euclideanDistance() = %f, want %f", got, tt.expected)
			}
		})
	}
}

package vector

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
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
	if !errors.Is(err, ErrDimensionMismatch) {
		t.Errorf("Insert with wrong dim: got %v, want ErrDimensionMismatch", err)
	}

	// Insert with correct dimension
	err = hnsw.Insert("v1", []float32{1.0, 0.0, 0.0})
	if err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	// Search with wrong dimension
	_, err = hnsw.Search([]float32{1.0, 0.0}, 1, 0)
	if !errors.Is(err, ErrDimensionMismatch) {
		t.Errorf("Search with wrong dim: got %v, want ErrDimensionMismatch", err)
	}
}

func TestHNSW_Delete(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{Dim: 3})

	require.NoError(t, hnsw.Insert("v1", []float32{1.0, 0.0, 0.0}))
	require.NoError(t, hnsw.Insert("v2", []float32{0.0, 1.0, 0.0}))
	require.NoError(t, hnsw.Insert("v3", []float32{0.0, 0.0, 1.0}))

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

	if len(results) != 0 {
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

func TestHNSW_SelectNeighbors_FewerCandidatesThanM(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          3,
		M:            16,
		EfConstruct:  200,
		DistanceType: DistanceEuclidean,
	})

	// Insert a few vectors
	require.NoError(t, hnsw.Insert("v1", []float32{1.0, 0.0, 0.0}))
	require.NoError(t, hnsw.Insert("v2", []float32{0.0, 1.0, 0.0}))

	// selectNeighbors with m > len(candidates) should return all candidates
	query := []float32{0.5, 0.5, 0.0}
	result := hnsw.selectNeighbors(query, []string{"v1", "v2"}, 10)
	if len(result) != 2 {
		t.Errorf("selectNeighbors returned %d neighbors, want 2 (all candidates)", len(result))
	}
}

func TestHNSW_SelectNeighbors_ExactM(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          3,
		M:            2,
		EfConstruct:  200,
		DistanceType: DistanceEuclidean,
	})

	require.NoError(t, hnsw.Insert("v1", []float32{1.0, 0.0, 0.0}))
	require.NoError(t, hnsw.Insert("v2", []float32{0.0, 1.0, 0.0}))

	query := []float32{0.5, 0.5, 0.0}
	result := hnsw.selectNeighbors(query, []string{"v1", "v2"}, 2)
	if len(result) != 2 {
		t.Errorf("selectNeighbors returned %d neighbors, want 2", len(result))
	}
}

func TestHNSW_SelectNeighbors_SelectsClosest(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          2,
		M:            16,
		EfConstruct:  200,
		DistanceType: DistanceEuclidean,
	})

	// Insert vectors at known positions
	require.NoError(t, hnsw.Insert("close", []float32{1.0, 0.0}))
	require.NoError(t, hnsw.Insert("medium", []float32{3.0, 0.0}))
	require.NoError(t, hnsw.Insert("far", []float32{10.0, 0.0}))

	query := []float32{0.0, 0.0}
	result := hnsw.selectNeighbors(query, []string{"close", "medium", "far"}, 2)
	if len(result) != 2 {
		t.Fatalf("selectNeighbors returned %d neighbors, want 2", len(result))
	}

	// The two closest should be "close" and "medium"
	resultSet := map[string]bool{result[0]: true, result[1]: true}
	if !resultSet["close"] {
		t.Error("expected 'close' in results")
	}
	if !resultSet["medium"] {
		t.Error("expected 'medium' in results")
	}
}

func TestHNSW_SelectNeighbors_SingleCandidate(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          2,
		M:            16,
		EfConstruct:  200,
		DistanceType: DistanceEuclidean,
	})

	require.NoError(t, hnsw.Insert("v1", []float32{1.0, 0.0}))

	query := []float32{0.0, 0.0}
	result := hnsw.selectNeighbors(query, []string{"v1"}, 5)
	if len(result) != 1 {
		t.Errorf("selectNeighbors returned %d neighbors, want 1", len(result))
	}
	if result[0] != "v1" {
		t.Errorf("result[0] = %q, want %q", result[0], "v1")
	}
}

func TestHNSW_SelectNeighbors_DuplicateDistances(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          2,
		M:            16,
		EfConstruct:  200,
		DistanceType: DistanceEuclidean,
	})

	// Insert vectors at equal distances from origin
	require.NoError(t, hnsw.Insert("v1", []float32{1.0, 0.0}))
	require.NoError(t, hnsw.Insert("v2", []float32{0.0, 1.0}))
	require.NoError(t, hnsw.Insert("v3", []float32{-1.0, 0.0}))
	require.NoError(t, hnsw.Insert("v4", []float32{0.0, -1.0}))

	query := []float32{0.0, 0.0}
	result := hnsw.selectNeighbors(query, []string{"v1", "v2", "v3", "v4"}, 2)
	if len(result) != 2 {
		t.Errorf("selectNeighbors returned %d neighbors, want 2", len(result))
	}
}

func TestHNSW_SelectNeighbors_EmptyCandidates(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          2,
		M:            16,
		EfConstruct:  200,
		DistanceType: DistanceEuclidean,
	})

	query := []float32{0.0, 0.0}
	result := hnsw.selectNeighbors(query, []string{}, 5)
	if len(result) != 0 {
		t.Errorf("selectNeighbors returned %d neighbors, want 0", len(result))
	}
}

func TestHNSW_SelectNeighbors_LargerSet(t *testing.T) {
	hnsw := NewHNSW(HNSWConfig{
		Dim:          2,
		M:            16,
		EfConstruct:  200,
		DistanceType: DistanceEuclidean,
	})

	// Insert 10 vectors at increasing distances
	for i := 0; i < 10; i++ {
		id := string(rune('A' + i))
		require.NoError(t, hnsw.Insert(id, []float32{float32(i + 1), 0.0}))
	}

	query := []float32{0.0, 0.0}
	candidates := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
	result := hnsw.selectNeighbors(query, candidates, 3)
	if len(result) != 3 {
		t.Fatalf("selectNeighbors returned %d, want 3", len(result))
	}

	// Should be the 3 closest: A(1,0), B(2,0), C(3,0)
	resultSet := map[string]bool{result[0]: true, result[1]: true, result[2]: true}
	if !resultSet["A"] || !resultSet["B"] || !resultSet["C"] {
		t.Errorf("expected A, B, C in results, got %v", result)
	}
}

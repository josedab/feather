// Example: Vector similarity search
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/feather-store/feather/sdk/go/feather"
)

const vectorDim = 128

func main() {
	client := feather.NewClient("http://localhost:8080", "", nil)
	ctx := context.Background()

	// Create a vector index
	fmt.Println("Creating vector index...")
	err := client.Vectors.CreateIndex(ctx, &feather.VectorIndex{
		Name:       "product-embeddings",
		Dimensions: vectorDim,
		Metric:     "cosine",
	})
	if err != nil {
		log.Printf("Index creation (may already exist): %v", err)
	} else {
		fmt.Println("Index created successfully")
	}

	// Generate sample product embeddings
	fmt.Println("\nUpserting vectors...")
	vectors := make(map[string][]float64)
	metadata := make(map[string]map[string]interface{})

	products := []struct {
		id       string
		category string
		price    float64
	}{
		{"prod:1", "electronics", 299.99},
		{"prod:2", "electronics", 149.99},
		{"prod:3", "clothing", 49.99},
		{"prod:4", "clothing", 79.99},
		{"prod:5", "home", 199.99},
		{"prod:6", "home", 89.99},
		{"prod:7", "electronics", 599.99},
		{"prod:8", "clothing", 129.99},
		{"prod:9", "home", 349.99},
		{"prod:10", "electronics", 999.99},
	}

	// Generate embeddings with category-based clustering
	categorySeeds := map[string]int64{
		"electronics": 42,
		"clothing":    123,
		"home":        456,
	}

	for _, p := range products {
		vectors[p.id] = generateEmbedding(vectorDim, categorySeeds[p.category])
		metadata[p.id] = map[string]interface{}{
			"category": p.category,
			"price":    p.price,
		}
	}

	err = client.Vectors.Upsert(ctx, "product-embeddings", vectors, metadata)
	if err != nil {
		log.Fatalf("Failed to upsert vectors: %v", err)
	}
	fmt.Printf("Upserted %d vectors\n", len(vectors))

	// Search for similar products
	fmt.Println("\nSearching for similar products...")

	// Search with an electronics-like query vector
	queryVector := generateEmbedding(vectorDim, categorySeeds["electronics"])

	results, err := client.Vectors.Search(ctx, "product-embeddings", queryVector, 5)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	fmt.Println("\nTop 5 similar products to electronics query:")
	for i, result := range results {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.ID, result.Score)
	}

	// Search with a clothing-like query vector
	queryVector = generateEmbedding(vectorDim, categorySeeds["clothing"])

	results, err = client.Vectors.Search(ctx, "product-embeddings", queryVector, 5)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	fmt.Println("\nTop 5 similar products to clothing query:")
	for i, result := range results {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.ID, result.Score)
	}

	fmt.Println("\nDone!")
}

// generateEmbedding creates a normalized random embedding with a seed for reproducibility
func generateEmbedding(dim int, seed int64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	vec := make([]float64, dim)

	// Generate random values
	var norm float64
	for i := range vec {
		vec[i] = rng.Float64()*2 - 1 // Random value between -1 and 1
		norm += vec[i] * vec[i]
	}

	// Add some noise to differentiate within category
	noise := rand.New(rand.NewSource(rand.Int63()))
	for i := range vec {
		vec[i] += noise.Float64() * 0.1 // 10% noise
	}

	// Normalize to unit length
	norm = math.Sqrt(norm)
	for i := range vec {
		vec[i] /= norm
	}

	return vec
}

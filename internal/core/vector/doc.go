// Package vector provides vector similarity search for embeddings.
//
// It implements an HNSW (Hierarchical Navigable Small World) index for
// efficient approximate nearest neighbor search on high-dimensional vectors.
// The package supports multiple distance metrics and configurable index
// parameters for tuning accuracy/speed tradeoffs.
//
// Key components:
//   - Store: Manages multiple vector indexes
//   - Index: HNSW index for a single vector collection
//   - SearchResult: Represents a nearest neighbor with distance score
//
// Example usage:
//
//	store := vector.NewStore(vector.StoreConfig{DataDir: "/data"})
//	store.CreateIndex("embeddings", 768, vector.DistanceCosine)
//	store.Upsert("embeddings", records)
//	results := store.Search("embeddings", queryVector, 10)
package vector

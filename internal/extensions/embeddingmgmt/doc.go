// Package embeddingmgmt provides end-to-end embedding lifecycle management:
// generate, store, version, index, and serve embeddings with automatic
// reindexing on model updates.
//
// Key components:
//   - Manager: Orchestrates the embedding lifecycle
//   - Index: HNSW/IVF vector index management
//   - ModelRegistry: Manages embedding model backends
package embeddingmgmt

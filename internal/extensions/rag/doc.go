// Package rag provides a native RAG (Retrieval-Augmented Generation) pipeline.
//
// It integrates document ingestion, chunking, embedding, and semantic retrieval
// into a unified pipeline for building RAG-powered applications on top of Feather.
//
// Key components:
//   - Pipeline: End-to-end RAG pipeline managing documents, chunks, and retrieval
//   - Chunker: Configurable text chunking with multiple strategies (fixed, sentence, paragraph)
//   - Indexer: In-memory vector index with cosine similarity search
//   - Embedder: Pluggable embedding interface with a local bag-of-words implementation
//
// Example usage:
//
//	pipeline := rag.NewPipeline(rag.DefaultPipelineConfig())
//	doc := &rag.Document{Content: "...", Source: "wiki"}
//	pipeline.Ingest(ctx, doc)
//	results, _ := pipeline.Retrieve(ctx, "search query", 5)
package rag

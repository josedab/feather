// Package llm provides LLM-powered feature generation pipelines.
//
// It enables automatic generation of embedding features from text data using
// various LLM providers (OpenAI, Ollama, HuggingFace). The package includes:
//
//   - Provider: Abstract interface for embedding generation with multiple backends
//   - Chunker: Text chunking strategies (fixed, semantic, recursive)
//   - Pipeline: End-to-end text-to-embedding feature generation
//   - Cache: Embedding cache with content-based deduplication
//
// # Basic Usage
//
// Create a pipeline with an embedding provider:
//
//	provider := llm.NewOpenAIProvider(llm.OpenAIConfig{
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	    Model:  "text-embedding-3-small",
//	})
//
//	pipeline := llm.NewPipeline(llm.PipelineConfig{
//	    Provider:    provider,
//	    ChunkSize:   512,
//	    ChunkMethod: llm.ChunkMethodSemantic,
//	})
//
//	// Generate embeddings from text
//	embeddings, err := pipeline.Process(ctx, "user:123", "text_content", longText)
//
// # Chunking Strategies
//
// The package supports multiple text chunking strategies:
//
//   - Fixed: Split text into fixed-size chunks
//   - Semantic: Split at sentence/paragraph boundaries
//   - Recursive: Hierarchical splitting with configurable separators
//
// # Provider Support
//
// Supported embedding providers:
//
//   - OpenAI: text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002
//   - Ollama: nomic-embed-text, mxbai-embed-large, all-minilm
//   - HuggingFace: sentence-transformers models via Inference API
//   - Local: TF-IDF based embeddings for development/testing
package llm

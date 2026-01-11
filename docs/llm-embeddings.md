# LLM-Powered Embeddings

> Generate and serve feature embeddings using large language models.

## Table of Contents

- [Overview](#overview)
- [Supported Providers](#supported-providers)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Text Chunking](#text-chunking)
- [Pipeline Architecture](#pipeline-architecture)
- [Performance](#performance)
- [Examples](#examples)

---

## Overview

The LLM package enables Feather to generate embeddings from text content using various LLM providers. This powers:

- **Semantic Search**: Find features by meaning, not just keywords
- **Feature Discovery**: Natural language queries for feature exploration
- **Content Embeddings**: Convert text features into dense vector representations
- **Similar Feature Detection**: Identify related features across groups

### Key Capabilities

| Capability | Description |
|------------|-------------|
| **Multi-Provider Support** | OpenAI, Ollama, HuggingFace, and local TF-IDF |
| **Intelligent Chunking** | Semantic and fixed-size text splitting |
| **Caching** | Deduplicated embedding cache |
| **Batch Processing** | Efficient batch embedding generation |
| **Pipeline Integration** | End-to-end text-to-vector pipelines |

---

## Supported Providers

### OpenAI

Industry-standard embeddings with excellent quality.

```yaml
llm:
  provider: openai
  config:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"  # or text-embedding-3-large, text-embedding-ada-002
    dimensions: 1536
    batch_size: 100
    timeout: "30s"
```

**Available Models:**

| Model | Dimensions | Max Tokens | Best For |
|-------|------------|------------|----------|
| `text-embedding-3-small` | 1536 | 8191 | Cost-effective general use |
| `text-embedding-3-large` | 3072 | 8191 | High-quality embeddings |
| `text-embedding-ada-002` | 1536 | 8191 | Legacy compatibility |

### Ollama (Local)

Run embedding models locally for privacy and low latency.

```yaml
llm:
  provider: ollama
  config:
    host: "http://localhost:11434"
    model: "nomic-embed-text"  # or mxbai-embed-large
    dimensions: 768
    timeout: "60s"
```

**Available Models:**

| Model | Dimensions | Notes |
|-------|------------|-------|
| `nomic-embed-text` | 768 | Fast, good quality |
| `mxbai-embed-large` | 1024 | Higher quality, larger |
| `all-minilm` | 384 | Lightweight, fast |

### HuggingFace

Use models from the HuggingFace Hub via Inference API.

```yaml
llm:
  provider: huggingface
  config:
    api_key: "${HF_API_KEY}"
    model: "sentence-transformers/all-MiniLM-L6-v2"
    dimensions: 384
    endpoint: "https://api-inference.huggingface.co"
    timeout: "30s"
```

**Popular Models:**

| Model | Dimensions | Notes |
|-------|------------|-------|
| `sentence-transformers/all-MiniLM-L6-v2` | 384 | Fast, lightweight |
| `sentence-transformers/all-mpnet-base-v2` | 768 | Better quality |
| `BAAI/bge-small-en-v1.5` | 384 | State-of-the-art compact |
| `BAAI/bge-large-en-v1.5` | 1024 | High quality |

### Local TF-IDF

Lightweight local embeddings for testing and simple use cases.

```yaml
llm:
  provider: tfidf
  config:
    max_features: 1000
    ngram_range: [1, 2]
    stop_words: "english"
```

---

## Configuration

### Full Configuration Example

```yaml
# configs/feather.yaml
llm:
  # Primary provider
  provider: openai
  config:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"
    dimensions: 1536

  # Chunking settings
  chunker:
    strategy: "semantic"  # or "fixed"
    max_chunk_size: 512
    overlap: 50
    separator: "\n\n"

  # Caching
  cache:
    enabled: true
    max_size: 10000
    ttl: "24h"

  # Rate limiting
  rate_limit:
    requests_per_minute: 3000
    tokens_per_minute: 1000000

  # Retry settings
  retry:
    max_attempts: 3
    initial_backoff: "1s"
    max_backoff: "30s"

  # Fallback provider (optional)
  fallback:
    provider: tfidf
    config:
      max_features: 500
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `FEATHER_LLM_PROVIDER` | Embedding provider (`openai`, `ollama`, `huggingface`, `tfidf`) |
| `FEATHER_LLM_API_KEY` | API key for cloud providers |
| `FEATHER_LLM_MODEL` | Model identifier |
| `FEATHER_LLM_DIMENSIONS` | Embedding dimensions |
| `OPENAI_API_KEY` | OpenAI API key (alternative) |
| `HF_API_KEY` | HuggingFace API key (alternative) |

---

## API Reference

### HTTP Endpoints

#### Generate Embeddings

```
POST /v1/embeddings
```

**Request Body:**

```json
{
  "texts": [
    "User clicked on product page",
    "Customer made a purchase",
    "Added item to cart"
  ],
  "model": "text-embedding-3-small"
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "embeddings": [
      {
        "index": 0,
        "values": [0.0123, -0.0456, 0.0789, ...],
        "tokens": 6
      },
      {
        "index": 1,
        "values": [0.0234, -0.0567, 0.0890, ...],
        "tokens": 4
      },
      {
        "index": 2,
        "values": [0.0345, -0.0678, 0.0901, ...],
        "tokens": 4
      }
    ],
    "model": "text-embedding-3-small",
    "usage": {
      "prompt_tokens": 14,
      "total_tokens": 14
    }
  },
  "request_id": "req-abc123"
}
```

#### Embed Feature Content

```
POST /v1/features/embed
```

Embed text content and store as a vector feature.

**Request Body:**

```json
{
  "entity_key": "doc:12345",
  "feature_name": "content_embedding",
  "text": "This is a product description with detailed specifications...",
  "chunk": true
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "entity_key": "doc:12345",
    "feature_name": "content_embedding",
    "dimensions": 1536,
    "chunks": 3,
    "tokens_used": 342
  },
  "request_id": "req-def456"
}
```

#### Semantic Feature Search

```
POST /v1/features/search/semantic
```

Find features by natural language description.

**Request Body:**

```json
{
  "query": "features related to user purchase behavior",
  "top_k": 10,
  "min_score": 0.7
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "results": [
      {
        "feature": "purchase_total_7d",
        "group": "user-engagement",
        "score": 0.92,
        "description": "Total purchase amount in the last 7 days"
      },
      {
        "feature": "purchase_count",
        "group": "user-transactions",
        "score": 0.88,
        "description": "Number of purchases made by user"
      }
    ]
  },
  "request_id": "req-ghi789"
}
```

---

## Text Chunking

The chunker splits large text into appropriately sized pieces for embedding.

### Strategies

#### Fixed-Size Chunking

Simple character-based splitting with overlap.

```yaml
chunker:
  strategy: "fixed"
  max_chunk_size: 500
  overlap: 50
```

**Behavior:**
- Splits text every `max_chunk_size` characters
- Maintains `overlap` characters between chunks
- Fast and predictable

#### Semantic Chunking

Intelligent splitting at sentence or paragraph boundaries.

```yaml
chunker:
  strategy: "semantic"
  max_chunk_size: 500
  overlap: 50
  separator: "\n\n"
```

**Behavior:**
- Prefers splitting at paragraph breaks (`\n\n`)
- Falls back to sentence boundaries
- Maintains semantic coherence
- Respects max size constraints

#### Recursive Hierarchical

Multi-level splitting for complex documents.

```yaml
chunker:
  strategy: "recursive"
  max_chunk_size: 500
  separators:
    - "\n\n"      # Paragraphs first
    - "\n"        # Then lines
    - ". "        # Then sentences
    - " "         # Finally words
```

### Chunking Examples

**Input text:**
```
Machine learning is transforming how we build software.
Neural networks can learn complex patterns from data.

Feature stores help ML teams manage and serve features efficiently.
They provide low-latency access to computed features.
```

**Semantic chunks (max 100 chars):**
```
Chunk 1: "Machine learning is transforming how we build software.
Neural networks can learn complex patterns from data."

Chunk 2: "Feature stores help ML teams manage and serve features efficiently.
They provide low-latency access to computed features."
```

---

## Pipeline Architecture

The embedding pipeline processes text through multiple stages:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Embedding Pipeline                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Input Text ──────┐                                             │
│                   ▼                                             │
│         ┌─────────────────┐                                     │
│         │    Chunker      │  Split into manageable pieces       │
│         └────────┬────────┘                                     │
│                  ▼                                              │
│         ┌─────────────────┐                                     │
│         │  Deduplication  │  Skip already-computed chunks       │
│         └────────┬────────┘                                     │
│                  ▼                                              │
│         ┌─────────────────┐                                     │
│         │   Batch Queue   │  Accumulate for efficient API calls │
│         └────────┬────────┘                                     │
│                  ▼                                              │
│         ┌─────────────────┐                                     │
│         │    Provider     │  Call embedding API (batched)       │
│         └────────┬────────┘                                     │
│                  ▼                                              │
│         ┌─────────────────┐                                     │
│         │     Cache       │  Store for future reuse             │
│         └────────┬────────┘                                     │
│                  ▼                                              │
│         ┌─────────────────┐                                     │
│         │   Aggregation   │  Combine chunk embeddings           │
│         └────────┬────────┘                                     │
│                  ▼                                              │
│  Output Vector ──┘                                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Pipeline Configuration

```yaml
pipeline:
  # Batching
  batch_size: 50
  batch_timeout: "100ms"

  # Chunk aggregation strategy
  aggregation: "mean"  # or "first", "max", "cls"

  # Concurrent processing
  workers: 4
  queue_size: 1000

  # Error handling
  on_error: "skip"  # or "fail", "fallback"
```

### Aggregation Strategies

When text is split into multiple chunks, embeddings must be combined:

| Strategy | Description |
|----------|-------------|
| `mean` | Average of all chunk embeddings (default) |
| `first` | Use only first chunk embedding |
| `max` | Element-wise maximum across chunks |
| `weighted` | Weight by chunk position (first chunks weighted higher) |

---

## Performance

### Latency Benchmarks

| Provider | Batch Size | Latency (P50) | Latency (P99) |
|----------|------------|---------------|---------------|
| OpenAI | 1 | 150ms | 300ms |
| OpenAI | 50 | 400ms | 800ms |
| Ollama (local) | 1 | 50ms | 100ms |
| Ollama (local) | 50 | 200ms | 400ms |
| TF-IDF | 1 | 1ms | 5ms |

### Cost Optimization

1. **Caching**: Enable embedding cache to avoid re-computing
2. **Batching**: Group requests for better throughput
3. **Model Selection**: Use smaller models when quality permits
4. **Chunking**: Optimize chunk size to reduce tokens

### Cache Effectiveness

```yaml
cache:
  enabled: true
  max_size: 100000  # Number of cached embeddings
  ttl: "168h"       # 1 week
  strategy: "lru"   # Eviction strategy
```

**Typical Hit Rates:**
- Feature descriptions: 95%+ (static content)
- User content: 40-60% (dynamic, but repeated)
- Unique content: 0% (no benefit)

---

## Examples

### Basic Embedding Generation

```go
package main

import (
    "context"
    "fmt"

    "github.com/your-org/feather/internal/llm"
)

func main() {
    // Create provider
    provider, err := llm.NewOpenAIProvider(llm.OpenAIConfig{
        APIKey:     os.Getenv("OPENAI_API_KEY"),
        Model:      "text-embedding-3-small",
        Dimensions: 1536,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Generate embedding
    embedding, err := provider.Embed(context.Background(), "user clicked on product")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Embedding dimensions: %d\n", len(embedding))
}
```

### Pipeline with Chunking

```go
package main

import (
    "context"

    "github.com/your-org/feather/internal/llm"
)

func main() {
    // Create chunker
    chunker := llm.NewSemanticChunker(llm.ChunkerConfig{
        MaxChunkSize: 500,
        Overlap:      50,
    })

    // Create pipeline
    pipeline := llm.NewPipeline(llm.PipelineConfig{
        Provider:    provider,
        Chunker:     chunker,
        Aggregation: "mean",
        BatchSize:   50,
    })

    // Process long text
    longText := `
        This is a very long document that needs to be split
        into multiple chunks for efficient embedding generation...
    `

    embedding, err := pipeline.Process(context.Background(), longText)
    if err != nil {
        log.Fatal(err)
    }

    // Store as feature
    store.Put(ctx, "doc:123", map[string]*domain.FeatureValue{
        "content_embedding": {
            Value:     embedding,
            Timestamp: time.Now().UnixNano(),
        },
    })
}
```

### Feature Discovery with Semantic Search

```go
package main

import (
    "context"
    "fmt"

    "github.com/your-org/feather/internal/llm"
)

func main() {
    // Create semantic search client
    search := llm.NewSemanticSearch(llm.SemanticSearchConfig{
        Provider:   provider,
        VectorStore: vectorStore,
        IndexName:   "feature_descriptions",
    })

    // Index all feature descriptions
    for _, group := range registry.ListGroups() {
        for _, feature := range group.Features {
            search.Index(context.Background(), llm.Document{
                ID:      feature.Name,
                Content: feature.Description,
                Metadata: map[string]string{
                    "group":     group.Name,
                    "data_type": string(feature.DataType),
                },
            })
        }
    }

    // Search by natural language
    results, err := search.Search(context.Background(),
        "features about user shopping behavior",
        10,
    )
    if err != nil {
        log.Fatal(err)
    }

    for _, result := range results {
        fmt.Printf("Feature: %s (score: %.2f)\n", result.ID, result.Score)
    }
}
```

---

## Further Reading

- [Discovery Guide](./discovery.md) - AI-powered feature discovery
- [Vector Search](./api-reference.md#vector-search) - Vector similarity API
- [Architecture Overview](./architecture.md) - System design

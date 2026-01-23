---
title: "ADR-0015: HNSW Vector Similarity"
sidebar_label: "0015: HNSW Vectors"
---

# ADR-0015: HNSW Algorithm for Vector Similarity Search

## Status

Accepted

## Context

Modern ML applications require serving **embedding vectors** alongside traditional features:
- User embeddings from collaborative filtering
- Product embeddings from transformers
- Text embeddings from sentence-BERT
- Image embeddings from CLIP

Common use cases:
1. **Similar item retrieval**: "Find products similar to this one"
2. **Personalization**: "Find users with similar preferences"
3. **Semantic search**: "Find features semantically related to this query"

These operations require **nearest neighbor search** over high-dimensional vectors (typically 128-1024 dimensions). Exact brute-force search is O(n) per query, which becomes prohibitive at scale:

| Vectors | Dimensions | Brute-force time |
|---------|------------|------------------|
| 100K | 256 | ~10ms |
| 1M | 256 | ~100ms |
| 10M | 256 | ~1s |

We needed an approximate nearest neighbor (ANN) algorithm that provides:
1. **Sub-millisecond queries**: Match our latency targets
2. **High recall**: >95% of true nearest neighbors found
3. **Dynamic updates**: Support real-time vector ingestion
4. **Memory efficiency**: Fit in available RAM

## Decision

We implement **HNSW (Hierarchical Navigable Small World)** for vector similarity search.

### Why HNSW?

HNSW provides the best tradeoff for our requirements:

| Algorithm | Query Time | Index Time | Memory | Dynamic Updates |
|-----------|------------|------------|--------|-----------------|
| **HNSW** | O(log n) | O(log n) | High | Yes |
| LSH | O(1) | O(n) | Low | Yes |
| IVF | O(n/k) | O(n) | Medium | Partial |
| KD-Tree | O(log n) | O(n log n) | Low | No |
| Annoy | O(log n) | O(n log n) | Medium | No |

HNSW excels because:
- **Logarithmic search**: Navigates hierarchical graph structure
- **Dynamic updates**: Add/remove vectors without full rebuild
- **Tunable accuracy**: Trade latency for recall via parameters
- **Battle-tested**: Used by Pinecone, Milvus, Weaviate, Qdrant

### Algorithm Overview

HNSW builds a multi-layer graph where:
- **Layer 0**: Contains all vectors (dense)
- **Layer 1+**: Contains subset of vectors (sparse, long-range connections)

```
Layer 2:  A ─────────────────────── B
          │                         │
Layer 1:  A ─── C ─────── D ─────── B
          │     │         │         │
Layer 0:  A─E─F─C─G─H─I─J─D─K─L─M─N─B
                          ↑
                     Query point
```

**Search process**:
1. Start at entry point in top layer
2. Greedily navigate to nearest neighbor
3. Descend to next layer, continue navigation
4. At layer 0, return k nearest neighbors found

### Parameters

| Parameter | Default | Effect |
|-----------|---------|--------|
| `M` | 16 | Connections per node. Higher = better recall, more memory |
| `efConstruction` | 200 | Build quality. Higher = better graph, slower indexing |
| `efSearch` | 100 | Query quality. Higher = better recall, slower queries |

**Tuning guidance**:
- Start with defaults
- Increase `efSearch` if recall is too low
- Increase `M` if recall plateaus despite high `efSearch`
- Decrease `efSearch` for faster queries (accept lower recall)

### Distance Metrics

```go
type DistanceMetric int

const (
    Cosine     DistanceMetric = iota  // 1 - cos(a,b), for normalized vectors
    Euclidean                          // L2 distance, for spatial data
    DotProduct                         // -dot(a,b), for max inner product
)
```

**Metric selection**:
- **Cosine**: Text embeddings, recommendation models
- **Euclidean**: Image embeddings, geometric data
- **DotProduct**: When vectors are pre-normalized and you want MIP

## Consequences

### Positive

- **Fast queries**: O(log n) with typical latency under 1ms for 1M vectors
- **High recall**: >95% recall achievable with tuning
- **Dynamic**: Add vectors in O(log n) without rebuild
- **Memory-efficient**: Graph structure is compact
- **No external dependency**: Pure Go implementation, embedded in Feather

### Negative

- **Memory overhead**: Graph structure adds ~100-200 bytes per vector
- **Approximate results**: Not guaranteed to find exact nearest neighbors
- **Parameter sensitivity**: Requires tuning for optimal performance
- **No filtering during search**: Must post-filter results (inefficient for selective queries)
- **Index not persistent by default**: Must serialize to disk explicitly

### Neutral

- **In-memory only**: Vectors must fit in RAM; no disk-based ANN
- **Single-node**: No distributed vector search (handled at application layer)

## Performance Characteristics

Benchmarks on 1M vectors, 256 dimensions:

| Operation | Time | Recall@10 |
|-----------|------|-----------|
| Search (ef=50) | 0.3ms | 92% |
| Search (ef=100) | 0.6ms | 96% |
| Search (ef=200) | 1.2ms | 98% |
| Insert | 0.5ms | N/A |
| Delete | 0.1ms | N/A |

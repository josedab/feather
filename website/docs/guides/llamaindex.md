---
sidebar_position: 11
title: LlamaIndex Integration
description: Use Feather as a vector store for LlamaIndex RAG applications.
---

# LlamaIndex Integration

Feather provides a native [LlamaIndex](https://www.llamaindex.ai/) vector store integration, enabling you to build RAG (Retrieval-Augmented Generation) applications with Feather as the vector backend.

## Installation

Install the Feather Python client with LlamaIndex support:

```bash
pip install feather-client[llamaindex]
```

This installs:
- `feather-client` - Feather Python SDK
- `llama-index-core` - LlamaIndex core library

## Quick Start

```python
from llama_index.core import VectorStoreIndex, SimpleDirectoryReader
from llama_index.embeddings.openai import OpenAIEmbedding

from feather_client import FeatherClient
from feather_client.integrations.llamaindex import FeatherVectorStore

# Initialize Feather client
client = FeatherClient("http://localhost:8080")

# Create vector store
vector_store = FeatherVectorStore(
    client=client,
    index_name="documents",
)

# Load and index documents
documents = SimpleDirectoryReader("./data").load_data()
index = VectorStoreIndex.from_documents(
    documents,
    vector_store=vector_store,
    embed_model=OpenAIEmbedding(),
)

# Query
query_engine = index.as_query_engine()
response = query_engine.query("What is Feather?")
print(response)
```

## Creating the Vector Index

Before using the vector store, create an index in Feather:

```bash
# Using CLI
feather-cli vectors index create documents -d 1536 -m cosine

# Or via API
curl -X POST http://localhost:8080/v1/vectors \
  -H "Content-Type: application/json" \
  -d '{"name": "documents", "dimensions": 1536, "metric": "cosine"}'
```

The dimension should match your embedding model:
- OpenAI `text-embedding-ada-002`: 1536
- OpenAI `text-embedding-3-small`: 1536
- OpenAI `text-embedding-3-large`: 3072
- Cohere `embed-english-v3.0`: 1024
- HuggingFace `BAAI/bge-small-en`: 384

## API Reference

### FeatherVectorStore

```python
from feather_client.integrations.llamaindex import FeatherVectorStore

vector_store = FeatherVectorStore(
    client: FeatherClient,           # Feather client instance
    index_name: str,                 # Vector index name
    text_key: str = "text",          # Metadata key for node text
)
```

### Methods

#### add

Add nodes to the vector store.

```python
node_ids = vector_store.add(
    nodes: List[BaseNode],  # LlamaIndex nodes with embeddings
)
```

**Returns:** `List[str]` - IDs of added nodes

**Example:**

```python
from llama_index.core.schema import TextNode

nodes = [
    TextNode(text="Hello world", embedding=[0.1, 0.2, ...]),
    TextNode(text="Goodbye world", embedding=[0.3, 0.4, ...]),
]
ids = vector_store.add(nodes)
print(f"Added {len(ids)} nodes")
```

#### query

Query the vector store.

```python
from llama_index.core.vector_stores import VectorStoreQuery

query = VectorStoreQuery(
    query_embedding=[0.1, 0.2, ...],
    similarity_top_k=10,
    filters=None,  # Optional metadata filters
)

result = vector_store.query(query)
```

**Returns:** `VectorStoreQueryResult` with:
- `nodes`: List of matching nodes
- `similarities`: List of similarity scores
- `ids`: List of node IDs

**Example:**

```python
from llama_index.core.vector_stores import VectorStoreQuery

query = VectorStoreQuery(
    query_embedding=embed_model.get_query_embedding("What is Feather?"),
    similarity_top_k=5,
)

result = vector_store.query(query)
for node, score in zip(result.nodes, result.similarities):
    print(f"Score: {score:.4f} - {node.text[:50]}...")
```

#### delete_nodes

Delete nodes by ID.

```python
vector_store.delete_nodes(
    node_ids: List[str],
    delete_from_docstore: bool = False,
)
```

### Properties

#### client

Access the underlying Feather client:

```python
feather_client = vector_store.client
```

## RAG Examples

### Basic Query Engine

```python
from llama_index.core import VectorStoreIndex, StorageContext
from llama_index.embeddings.openai import OpenAIEmbedding
from llama_index.llms.openai import OpenAI

from feather_client import FeatherClient
from feather_client.integrations.llamaindex import FeatherVectorStore

# Setup
client = FeatherClient("http://localhost:8080")
vector_store = FeatherVectorStore(client=client, index_name="knowledge")

# Create index from existing vector store
storage_context = StorageContext.from_defaults(vector_store=vector_store)
index = VectorStoreIndex.from_vector_store(
    vector_store,
    embed_model=OpenAIEmbedding(),
    storage_context=storage_context,
)

# Create query engine
query_engine = index.as_query_engine(
    llm=OpenAI(model="gpt-4"),
    similarity_top_k=3,
)

# Query
response = query_engine.query("Explain the architecture of Feather")
print(response)
```

### Chat Engine with Memory

```python
from llama_index.core.memory import ChatMemoryBuffer

# Create chat engine with memory
memory = ChatMemoryBuffer.from_defaults(token_limit=3000)
chat_engine = index.as_chat_engine(
    chat_mode="context",
    memory=memory,
    llm=OpenAI(model="gpt-4"),
)

# Multi-turn conversation
response1 = chat_engine.chat("What is Feather?")
print(response1)

response2 = chat_engine.chat("How fast is it?")
print(response2)
```

### Document Ingestion Pipeline

```python
from llama_index.core import SimpleDirectoryReader
from llama_index.core.node_parser import SentenceSplitter
from llama_index.core.ingestion import IngestionPipeline
from llama_index.embeddings.openai import OpenAIEmbedding

from feather_client import FeatherClient
from feather_client.integrations.llamaindex import FeatherVectorStore

# Setup
client = FeatherClient("http://localhost:8080")
vector_store = FeatherVectorStore(client=client, index_name="docs")

# Create ingestion pipeline
pipeline = IngestionPipeline(
    transformations=[
        SentenceSplitter(chunk_size=512, chunk_overlap=50),
        OpenAIEmbedding(),
    ],
    vector_store=vector_store,
)

# Load and ingest documents
documents = SimpleDirectoryReader("./docs").load_data()
nodes = pipeline.run(documents=documents)
print(f"Ingested {len(nodes)} nodes")
```

### Hybrid Search with Features

Combine vector search with Feather feature retrieval:

```python
from feather_client import FeatherClient
from feather_client.integrations.llamaindex import FeatherVectorStore
from llama_index.core.vector_stores import VectorStoreQuery

client = FeatherClient("http://localhost:8080")
vector_store = FeatherVectorStore(client=client, index_name="products")

# Vector search for relevant products
query = VectorStoreQuery(
    query_embedding=embed_model.get_query_embedding(user_query),
    similarity_top_k=10,
)
result = vector_store.query(query)

# Get ML features for reranking
product_ids = [node.metadata["product_id"] for node in result.nodes]
features = client.get_features_batch(
    entities=[f"product:{pid}" for pid in product_ids],
    features=["click_through_rate", "conversion_rate", "popularity_score"],
)

# Rerank using features
ranked_products = rerank_by_features(result.nodes, result.similarities, features)
```

### Custom Retriever

```python
from llama_index.core.retrievers import VectorIndexRetriever
from llama_index.core.query_engine import RetrieverQueryEngine
from llama_index.core.postprocessor import SimilarityPostprocessor

# Create custom retriever
retriever = VectorIndexRetriever(
    index=index,
    similarity_top_k=10,
)

# Add postprocessor to filter by score
postprocessor = SimilarityPostprocessor(similarity_cutoff=0.7)

# Create query engine with custom retriever
query_engine = RetrieverQueryEngine(
    retriever=retriever,
    node_postprocessors=[postprocessor],
)

response = query_engine.query("What are the key features?")
```

## Metadata Filtering

Filter search results by metadata:

```python
from llama_index.core.vector_stores import (
    VectorStoreQuery,
    MetadataFilters,
    MetadataFilter,
    FilterOperator,
)

# Add nodes with metadata
nodes = [
    TextNode(
        text="Tech article 1",
        metadata={"category": "tech", "year": 2024},
        embedding=[...],
    ),
    TextNode(
        text="Science article",
        metadata={"category": "science", "year": 2023},
        embedding=[...],
    ),
]
vector_store.add(nodes)

# Query with filters
filters = MetadataFilters(
    filters=[
        MetadataFilter(key="category", value="tech", operator=FilterOperator.EQ),
    ]
)

query = VectorStoreQuery(
    query_embedding=[...],
    similarity_top_k=5,
    filters=filters,
)

result = vector_store.query(query)
```

## Performance Tips

### 1. Batch Node Addition

```python
# Good: Add all nodes at once
vector_store.add(nodes)

# Bad: Add one at a time
for node in nodes:
    vector_store.add([node])
```

### 2. Reuse Client Connection

```python
# Good: Share client across vector stores
client = FeatherClient("http://localhost:8080")
store1 = FeatherVectorStore(client=client, index_name="docs")
store2 = FeatherVectorStore(client=client, index_name="products")

# Bad: Create new client per store
store1 = FeatherVectorStore(client=FeatherClient(...), ...)
store2 = FeatherVectorStore(client=FeatherClient(...), ...)
```

### 3. Appropriate Top-K Values

```python
# Start with smaller k, increase if needed
query = VectorStoreQuery(
    query_embedding=embedding,
    similarity_top_k=5,  # Start small
)

# Avoid very large k values for latency-sensitive applications
```

### 4. Use Ingestion Pipeline for Large Datasets

```python
from llama_index.core.ingestion import IngestionPipeline

pipeline = IngestionPipeline(
    transformations=[...],
    vector_store=vector_store,
)

# Process in batches automatically
nodes = pipeline.run(documents=documents, show_progress=True)
```

## Troubleshooting

### "Index not found"

Create the index before using the vector store:

```bash
feather-cli vectors index create my-index -d 1536 -m cosine
```

### Dimension Mismatch

Ensure index dimensions match your embedding model:

```python
# Check embedding dimensions
test_embedding = embed_model.get_query_embedding("test")
print(f"Embedding dimension: {len(test_embedding)}")

# Create matching index
# feather-cli vectors index create my-index -d {len(test_embedding)}
```

### Connection Errors

Verify Feather is running and accessible:

```bash
curl http://localhost:8080/health
feather-cli health -s http://localhost:8080
```

### Empty Query Results

1. Verify nodes were added: check the Feather logs or UI
2. Lower the `similarity_top_k` value if using filters
3. Check that query embeddings use the same model as document embeddings
4. Verify metadata filter values match stored metadata exactly

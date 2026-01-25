---
sidebar_position: 10
title: LangChain Integration
description: Use Feather as a vector store for LangChain RAG applications.
---

# LangChain Integration

Feather provides a native [LangChain](https://langchain.com/) vector store integration, enabling you to use Feather for Retrieval-Augmented Generation (RAG) applications. No separate vector database needed—Feather handles both feature serving and vector search.

## Installation

Install the Feather Python client with LangChain support:

```bash
pip install feather-client[langchain]
```

This installs:
- `feather-client` - Feather Python SDK
- `langchain-core` - LangChain core library

## Quick Start

```python
from feather_client import FeatherClient
from feather_client.integrations.langchain import FeatherVectorStore
from langchain_openai import OpenAIEmbeddings

# Initialize Feather client
client = FeatherClient("http://localhost:8080")

# Create vector store with OpenAI embeddings
embeddings = OpenAIEmbeddings()
vector_store = FeatherVectorStore(
    client=client,
    index_name="documents",
    embedding=embeddings,
)

# Add documents
texts = [
    "Feather is a high-performance feature store.",
    "It provides sub-millisecond latency for ML inference.",
    "Vector search is built-in with HNSW indexing.",
]
vector_store.add_texts(texts)

# Search
results = vector_store.similarity_search("What is Feather?", k=2)
for doc in results:
    print(doc.page_content)
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
- Sentence Transformers `all-MiniLM-L6-v2`: 384

## API Reference

### FeatherVectorStore

```python
from feather_client.integrations.langchain import FeatherVectorStore

vector_store = FeatherVectorStore(
    client: FeatherClient,           # Feather client instance
    index_name: str,                 # Vector index name
    embedding: Embeddings = None,    # LangChain embeddings (optional)
    text_key: str = "text",          # Metadata key for document text
)
```

### Methods

#### add_texts

Add texts to the vector store.

```python
ids = vector_store.add_texts(
    texts: List[str],                # Texts to embed and store
    metadatas: List[dict] = None,    # Optional metadata for each text
    ids: List[str] = None,           # Optional IDs (auto-generated if not provided)
)
```

**Example:**

```python
texts = ["First document", "Second document"]
metadatas = [
    {"source": "file1.txt", "page": 1},
    {"source": "file2.txt", "page": 1},
]
ids = vector_store.add_texts(texts, metadatas=metadatas)
print(f"Added {len(ids)} documents")
```

#### add_documents

Add LangChain Document objects.

```python
from langchain_core.documents import Document

docs = [
    Document(page_content="Hello world", metadata={"source": "greeting"}),
    Document(page_content="Goodbye world", metadata={"source": "farewell"}),
]
ids = vector_store.add_documents(docs)
```

#### similarity_search

Search for similar documents.

```python
results = vector_store.similarity_search(
    query: str,           # Search query
    k: int = 4,           # Number of results
    filter: dict = None,  # Metadata filter (optional)
)
```

**Returns:** `List[Document]`

**Example:**

```python
results = vector_store.similarity_search("machine learning", k=5)
for doc in results:
    print(f"Content: {doc.page_content}")
    print(f"Metadata: {doc.metadata}")
```

#### similarity_search_with_score

Search with relevance scores.

```python
results = vector_store.similarity_search_with_score(
    query: str,
    k: int = 4,
)
```

**Returns:** `List[Tuple[Document, float]]`

**Example:**

```python
results = vector_store.similarity_search_with_score("machine learning", k=5)
for doc, score in results:
    print(f"Score: {score:.4f} - {doc.page_content[:50]}...")
```

#### delete

Delete documents by ID.

```python
success = vector_store.delete(ids: List[str])
```

#### from_texts (class method)

Create a vector store and add texts in one step.

```python
vector_store = FeatherVectorStore.from_texts(
    texts: List[str],
    embedding: Embeddings,
    metadatas: List[dict] = None,
    client: FeatherClient = None,
    index_name: str = "documents",
    **kwargs,
)
```

## Async Support

For async applications, use `AsyncFeatherVectorStore`:

```python
from feather_client.integrations.langchain import AsyncFeatherVectorStore

async def main():
    vector_store = AsyncFeatherVectorStore(
        client=client,
        index_name="documents",
        embedding=embeddings,
    )

    # Async add
    await vector_store.aadd_texts(["Hello", "World"])

    # Async search
    results = await vector_store.asimilarity_search("greeting", k=2)
```

## RAG Examples

### Basic RAG Chain

```python
from langchain_openai import ChatOpenAI, OpenAIEmbeddings
from langchain_core.prompts import ChatPromptTemplate
from langchain_core.runnables import RunnablePassthrough
from langchain_core.output_parsers import StrOutputParser

from feather_client import FeatherClient
from feather_client.integrations.langchain import FeatherVectorStore

# Setup
client = FeatherClient("http://localhost:8080")
embeddings = OpenAIEmbeddings()
vector_store = FeatherVectorStore(
    client=client,
    index_name="knowledge_base",
    embedding=embeddings,
)

# Create retriever
retriever = vector_store.as_retriever(search_kwargs={"k": 3})

# Create RAG chain
template = """Answer based on the following context:
{context}

Question: {question}
"""
prompt = ChatPromptTemplate.from_template(template)
llm = ChatOpenAI(model="gpt-4")

chain = (
    {"context": retriever, "question": RunnablePassthrough()}
    | prompt
    | llm
    | StrOutputParser()
)

# Query
response = chain.invoke("What is Feather?")
print(response)
```

### Conversational RAG

```python
from langchain.chains import ConversationalRetrievalChain
from langchain.memory import ConversationBufferMemory

memory = ConversationBufferMemory(
    memory_key="chat_history",
    return_messages=True,
)

chain = ConversationalRetrievalChain.from_llm(
    llm=ChatOpenAI(),
    retriever=vector_store.as_retriever(),
    memory=memory,
)

# Multi-turn conversation
response1 = chain.invoke({"question": "What is Feather?"})
response2 = chain.invoke({"question": "How fast is it?"})
```

### Document Ingestion Pipeline

```python
from langchain_community.document_loaders import DirectoryLoader, TextLoader
from langchain.text_splitter import RecursiveCharacterTextSplitter

# Load documents
loader = DirectoryLoader("./docs", glob="**/*.md", loader_cls=TextLoader)
documents = loader.load()

# Split into chunks
splitter = RecursiveCharacterTextSplitter(
    chunk_size=1000,
    chunk_overlap=200,
)
chunks = splitter.split_documents(documents)

# Add to Feather
vector_store.add_documents(chunks)
print(f"Ingested {len(chunks)} chunks")
```

### Hybrid Search with Features

Combine vector search with feature retrieval:

```python
from feather_client import FeatherClient
from feather_client.integrations.langchain import FeatherVectorStore

client = FeatherClient("http://localhost:8080")

# Vector search for relevant products
vector_store = FeatherVectorStore(
    client=client,
    index_name="product_descriptions",
    embedding=embeddings,
)
similar_products = vector_store.similarity_search(user_query, k=10)

# Get ML features for ranking
product_ids = [doc.metadata["product_id"] for doc in similar_products]
features = client.get_features_batch(
    entities=[f"product:{pid}" for pid in product_ids],
    features=["click_through_rate", "conversion_rate", "popularity_score"],
)

# Re-rank using features
ranked_products = rank_by_features(similar_products, features)
```

## Metadata Filtering

Filter search results by metadata:

```python
# Add documents with metadata
vector_store.add_texts(
    texts=["Doc 1", "Doc 2", "Doc 3"],
    metadatas=[
        {"category": "tech", "year": 2024},
        {"category": "science", "year": 2023},
        {"category": "tech", "year": 2023},
    ],
)

# Search with filter
results = vector_store.similarity_search(
    "technology",
    k=5,
    filter={"category": "tech"},
)
```

## Performance Tips

### 1. Batch Operations

```python
# Good: Batch add
vector_store.add_texts(texts, batch_size=100)

# Bad: One at a time
for text in texts:
    vector_store.add_texts([text])
```

### 2. Reuse Client

```python
# Good: Share client
client = FeatherClient("http://localhost:8080")
store1 = FeatherVectorStore(client=client, index_name="docs")
store2 = FeatherVectorStore(client=client, index_name="products")

# Bad: New client per store
store1 = FeatherVectorStore(client=FeatherClient(...), ...)
store2 = FeatherVectorStore(client=FeatherClient(...), ...)
```

### 3. Appropriate k Values

```python
# Start small, increase if needed
results = vector_store.similarity_search(query, k=5)

# Avoid overly large k
# results = vector_store.similarity_search(query, k=1000)  # Slow
```

### 4. Use Async for Concurrent Operations

```python
import asyncio
from feather_client.integrations.langchain import AsyncFeatherVectorStore

async def search_multiple(queries):
    tasks = [
        vector_store.asimilarity_search(q, k=3)
        for q in queries
    ]
    return await asyncio.gather(*tasks)
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
test_embedding = embeddings.embed_query("test")
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

### Slow Searches

1. Check index size: very large indexes may need tuning
2. Reduce `k` value
3. Consider filtering to narrow search scope
4. Use async for concurrent queries

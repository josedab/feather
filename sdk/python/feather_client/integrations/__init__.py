"""Feather integrations with popular LLM frameworks.

This module provides vector store implementations for:
- LangChain: `FeatherVectorStore` for RAG applications
- LlamaIndex: `FeatherVectorStore` for document retrieval

Example (LangChain):
    >>> from feather_client.integrations.langchain import FeatherVectorStore
    >>> from langchain_openai import OpenAIEmbeddings
    >>>
    >>> vectorstore = FeatherVectorStore(
    ...     feather_url="http://localhost:8080",
    ...     index_name="documents",
    ...     embedding=OpenAIEmbeddings()
    ... )
    >>> docs = vectorstore.similarity_search("What is machine learning?", k=5)

Example (LlamaIndex):
    >>> from feather_client.integrations.llamaindex import FeatherVectorStore
    >>> from llama_index.core import VectorStoreIndex
    >>>
    >>> vector_store = FeatherVectorStore(
    ...     feather_url="http://localhost:8080",
    ...     index_name="documents"
    ... )
    >>> index = VectorStoreIndex.from_vector_store(vector_store)
"""

__all__ = []

# Lazy imports to avoid requiring optional dependencies
def __getattr__(name: str):
    if name == "langchain":
        from feather_client.integrations import langchain
        return langchain
    elif name == "llamaindex":
        from feather_client.integrations import llamaindex
        return llamaindex
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")

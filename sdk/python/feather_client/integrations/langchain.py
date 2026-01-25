"""LangChain integration for Feather Vector Store.

This module provides a LangChain-compatible VectorStore implementation
that uses Feather's vector similarity search capabilities.

Example:
    >>> from feather_client.integrations.langchain import FeatherVectorStore
    >>> from langchain_openai import OpenAIEmbeddings
    >>>
    >>> # Create vector store
    >>> vectorstore = FeatherVectorStore(
    ...     feather_url="http://localhost:8080",
    ...     index_name="documents",
    ...     embedding=OpenAIEmbeddings()
    ... )
    >>>
    >>> # Add documents
    >>> vectorstore.add_texts(
    ...     texts=["Hello world", "Machine learning is great"],
    ...     metadatas=[{"source": "doc1"}, {"source": "doc2"}]
    ... )
    >>>
    >>> # Search
    >>> docs = vectorstore.similarity_search("Hello", k=5)
"""

from __future__ import annotations

import uuid
from typing import Any, Iterable, List, Optional, Tuple, Type

try:
    from langchain_core.documents import Document
    from langchain_core.embeddings import Embeddings
    from langchain_core.vectorstores import VectorStore
except ImportError as e:
    raise ImportError(
        "LangChain is required for this integration. "
        "Install with: pip install feather-client[langchain]"
    ) from e

from feather_client import FeatherClient, AsyncFeatherClient


class FeatherVectorStore(VectorStore):
    """LangChain VectorStore implementation backed by Feather.

    This class provides a drop-in replacement for other LangChain vector stores,
    allowing you to use Feather's high-performance vector search in your RAG
    applications.

    Args:
        feather_url: URL of the Feather server (e.g., "http://localhost:8080")
        index_name: Name of the vector index to use
        embedding: LangChain Embeddings instance for encoding text
        api_key: Optional API key for authentication
        text_key: Metadata key to store document text (default: "text")

    Example:
        >>> from langchain_openai import OpenAIEmbeddings
        >>>
        >>> store = FeatherVectorStore(
        ...     feather_url="http://localhost:8080",
        ...     index_name="my-docs",
        ...     embedding=OpenAIEmbeddings()
        ... )
        >>> store.add_texts(["Hello world"])
        >>> docs = store.similarity_search("Hello")
    """

    def __init__(
        self,
        feather_url: str,
        index_name: str,
        embedding: Embeddings,
        api_key: Optional[str] = None,
        text_key: str = "text",
    ) -> None:
        self._feather_url = feather_url
        self._index_name = index_name
        self._embedding = embedding
        self._api_key = api_key
        self._text_key = text_key
        self._client = FeatherClient(feather_url, api_key=api_key)

    @property
    def embeddings(self) -> Optional[Embeddings]:
        """Return the embedding function."""
        return self._embedding

    def add_texts(
        self,
        texts: Iterable[str],
        metadatas: Optional[List[dict]] = None,
        ids: Optional[List[str]] = None,
        **kwargs: Any,
    ) -> List[str]:
        """Add texts to the vector store.

        Args:
            texts: Iterable of strings to add
            metadatas: Optional list of metadata dicts for each text
            ids: Optional list of IDs for each text
            **kwargs: Additional arguments (ignored)

        Returns:
            List of IDs for the added texts
        """
        texts_list = list(texts)
        if not texts_list:
            return []

        # Generate IDs if not provided
        if ids is None:
            ids = [str(uuid.uuid4()) for _ in texts_list]

        # Generate embeddings
        embeddings = self._embedding.embed_documents(texts_list)

        # Prepare metadata
        if metadatas is None:
            metadatas = [{} for _ in texts_list]

        # Add text to metadata
        for i, (text, metadata) in enumerate(zip(texts_list, metadatas)):
            metadatas[i] = {**metadata, self._text_key: text}

        # Upsert to Feather
        vectors = {id_: emb for id_, emb in zip(ids, embeddings)}
        metadata_dict = {id_: meta for id_, meta in zip(ids, metadatas)}

        self._client.vectors.upsert(
            index_name=self._index_name,
            vectors=vectors,
            metadata=metadata_dict,
        )

        return ids

    def add_documents(
        self,
        documents: List[Document],
        ids: Optional[List[str]] = None,
        **kwargs: Any,
    ) -> List[str]:
        """Add documents to the vector store.

        Args:
            documents: List of Document objects to add
            ids: Optional list of IDs
            **kwargs: Additional arguments

        Returns:
            List of IDs for the added documents
        """
        texts = [doc.page_content for doc in documents]
        metadatas = [doc.metadata for doc in documents]
        return self.add_texts(texts=texts, metadatas=metadatas, ids=ids, **kwargs)

    def similarity_search(
        self,
        query: str,
        k: int = 4,
        filter: Optional[dict] = None,
        **kwargs: Any,
    ) -> List[Document]:
        """Search for documents similar to the query.

        Args:
            query: Query string
            k: Number of results to return
            filter: Optional metadata filter (not yet supported)
            **kwargs: Additional arguments

        Returns:
            List of Document objects
        """
        docs_and_scores = self.similarity_search_with_score(query, k=k, filter=filter)
        return [doc for doc, _ in docs_and_scores]

    def similarity_search_with_score(
        self,
        query: str,
        k: int = 4,
        filter: Optional[dict] = None,
        **kwargs: Any,
    ) -> List[Tuple[Document, float]]:
        """Search with relevance scores.

        Args:
            query: Query string
            k: Number of results to return
            filter: Optional metadata filter (not yet supported)
            **kwargs: Additional arguments

        Returns:
            List of (Document, score) tuples
        """
        # Generate query embedding
        query_embedding = self._embedding.embed_query(query)

        # Search in Feather
        results = self._client.vectors.search(
            index_name=self._index_name,
            vector=query_embedding,
            top_k=k,
        )

        # Convert to Documents
        docs_with_scores = []
        for result in results:
            metadata = result.metadata or {}
            text = metadata.pop(self._text_key, "")
            doc = Document(page_content=text, metadata=metadata)
            docs_with_scores.append((doc, result.score))

        return docs_with_scores

    def similarity_search_by_vector(
        self,
        embedding: List[float],
        k: int = 4,
        filter: Optional[dict] = None,
        **kwargs: Any,
    ) -> List[Document]:
        """Search by embedding vector.

        Args:
            embedding: Query embedding vector
            k: Number of results to return
            filter: Optional metadata filter
            **kwargs: Additional arguments

        Returns:
            List of Document objects
        """
        results = self._client.vectors.search(
            index_name=self._index_name,
            vector=embedding,
            top_k=k,
        )

        docs = []
        for result in results:
            metadata = result.metadata or {}
            text = metadata.pop(self._text_key, "")
            docs.append(Document(page_content=text, metadata=metadata))

        return docs

    def delete(
        self,
        ids: Optional[List[str]] = None,
        **kwargs: Any,
    ) -> Optional[bool]:
        """Delete documents by ID.

        Args:
            ids: List of document IDs to delete
            **kwargs: Additional arguments

        Returns:
            True if deletion was successful
        """
        if ids is None:
            return False

        for id_ in ids:
            self._client.vectors.delete(
                index_name=self._index_name,
                vector_id=id_,
            )

        return True

    @classmethod
    def from_texts(
        cls: Type["FeatherVectorStore"],
        texts: List[str],
        embedding: Embeddings,
        metadatas: Optional[List[dict]] = None,
        feather_url: str = "http://localhost:8080",
        index_name: str = "langchain",
        api_key: Optional[str] = None,
        **kwargs: Any,
    ) -> "FeatherVectorStore":
        """Create a FeatherVectorStore from texts.

        Args:
            texts: List of texts to add
            embedding: Embedding function
            metadatas: Optional metadata for each text
            feather_url: Feather server URL
            index_name: Name of the index
            api_key: Optional API key
            **kwargs: Additional arguments

        Returns:
            FeatherVectorStore instance with texts added
        """
        store = cls(
            feather_url=feather_url,
            index_name=index_name,
            embedding=embedding,
            api_key=api_key,
        )
        store.add_texts(texts=texts, metadatas=metadatas)
        return store

    @classmethod
    def from_documents(
        cls: Type["FeatherVectorStore"],
        documents: List[Document],
        embedding: Embeddings,
        feather_url: str = "http://localhost:8080",
        index_name: str = "langchain",
        api_key: Optional[str] = None,
        **kwargs: Any,
    ) -> "FeatherVectorStore":
        """Create a FeatherVectorStore from documents.

        Args:
            documents: List of Document objects
            embedding: Embedding function
            feather_url: Feather server URL
            index_name: Name of the index
            api_key: Optional API key
            **kwargs: Additional arguments

        Returns:
            FeatherVectorStore instance with documents added
        """
        texts = [doc.page_content for doc in documents]
        metadatas = [doc.metadata for doc in documents]
        return cls.from_texts(
            texts=texts,
            embedding=embedding,
            metadatas=metadatas,
            feather_url=feather_url,
            index_name=index_name,
            api_key=api_key,
            **kwargs,
        )


class AsyncFeatherVectorStore(FeatherVectorStore):
    """Async version of FeatherVectorStore for LangChain.

    This class provides async methods for use with async LangChain chains.

    Example:
        >>> store = AsyncFeatherVectorStore(
        ...     feather_url="http://localhost:8080",
        ...     index_name="docs",
        ...     embedding=OpenAIEmbeddings()
        ... )
        >>> docs = await store.asimilarity_search("query")
    """

    def __init__(
        self,
        feather_url: str,
        index_name: str,
        embedding: Embeddings,
        api_key: Optional[str] = None,
        text_key: str = "text",
    ) -> None:
        super().__init__(
            feather_url=feather_url,
            index_name=index_name,
            embedding=embedding,
            api_key=api_key,
            text_key=text_key,
        )
        self._async_client = AsyncFeatherClient(feather_url, api_key=api_key)

    async def aadd_texts(
        self,
        texts: Iterable[str],
        metadatas: Optional[List[dict]] = None,
        ids: Optional[List[str]] = None,
        **kwargs: Any,
    ) -> List[str]:
        """Async version of add_texts."""
        texts_list = list(texts)
        if not texts_list:
            return []

        if ids is None:
            ids = [str(uuid.uuid4()) for _ in texts_list]

        # Generate embeddings (may be async in future)
        embeddings = self._embedding.embed_documents(texts_list)

        if metadatas is None:
            metadatas = [{} for _ in texts_list]

        for i, (text, metadata) in enumerate(zip(texts_list, metadatas)):
            metadatas[i] = {**metadata, self._text_key: text}

        vectors = {id_: emb for id_, emb in zip(ids, embeddings)}
        metadata_dict = {id_: meta for id_, meta in zip(ids, metadatas)}

        await self._async_client.vectors.upsert(
            index_name=self._index_name,
            vectors=vectors,
            metadata=metadata_dict,
        )

        return ids

    async def asimilarity_search(
        self,
        query: str,
        k: int = 4,
        filter: Optional[dict] = None,
        **kwargs: Any,
    ) -> List[Document]:
        """Async version of similarity_search."""
        docs_and_scores = await self.asimilarity_search_with_score(query, k=k, filter=filter)
        return [doc for doc, _ in docs_and_scores]

    async def asimilarity_search_with_score(
        self,
        query: str,
        k: int = 4,
        filter: Optional[dict] = None,
        **kwargs: Any,
    ) -> List[Tuple[Document, float]]:
        """Async version of similarity_search_with_score."""
        query_embedding = self._embedding.embed_query(query)

        results = await self._async_client.vectors.search(
            index_name=self._index_name,
            vector=query_embedding,
            top_k=k,
        )

        docs_with_scores = []
        for result in results:
            metadata = result.metadata or {}
            text = metadata.pop(self._text_key, "")
            doc = Document(page_content=text, metadata=metadata)
            docs_with_scores.append((doc, result.score))

        return docs_with_scores

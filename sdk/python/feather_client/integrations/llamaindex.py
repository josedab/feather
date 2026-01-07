"""LlamaIndex integration for Feather Vector Store.

This module provides a LlamaIndex-compatible VectorStore implementation
that uses Feather's vector similarity search capabilities.

Example:
    >>> from feather_client.integrations.llamaindex import FeatherVectorStore
    >>> from llama_index.core import VectorStoreIndex, StorageContext
    >>>
    >>> # Create vector store
    >>> vector_store = FeatherVectorStore(
    ...     feather_url="http://localhost:8080",
    ...     index_name="documents"
    ... )
    >>>
    >>> # Create index from vector store
    >>> storage_context = StorageContext.from_defaults(vector_store=vector_store)
    >>> index = VectorStoreIndex.from_documents(
    ...     documents,
    ...     storage_context=storage_context
    ... )
    >>>
    >>> # Query
    >>> query_engine = index.as_query_engine()
    >>> response = query_engine.query("What is machine learning?")
"""

from __future__ import annotations

import uuid
from typing import Any, Dict, List, Optional

try:
    from llama_index.core.schema import BaseNode, TextNode
    from llama_index.core.vector_stores.types import (
        BasePydanticVectorStore,
        VectorStoreQuery,
        VectorStoreQueryResult,
    )
except ImportError as e:
    raise ImportError(
        "LlamaIndex is required for this integration. "
        "Install with: pip install feather-client[llamaindex]"
    ) from e

from feather_client import FeatherClient, AsyncFeatherClient


class FeatherVectorStore(BasePydanticVectorStore):
    """LlamaIndex VectorStore implementation backed by Feather.

    This class provides a drop-in replacement for other LlamaIndex vector stores,
    allowing you to use Feather's high-performance vector search for document
    retrieval in your RAG applications.

    Args:
        feather_url: URL of the Feather server (e.g., "http://localhost:8080")
        index_name: Name of the vector index to use
        api_key: Optional API key for authentication

    Example:
        >>> from llama_index.core import VectorStoreIndex
        >>>
        >>> vector_store = FeatherVectorStore(
        ...     feather_url="http://localhost:8080",
        ...     index_name="my-docs"
        ... )
        >>> index = VectorStoreIndex.from_vector_store(vector_store)
    """

    # Pydantic fields
    feather_url: str
    index_name: str
    api_key: Optional[str] = None

    # Non-serialized attributes
    _client: Optional[FeatherClient] = None
    _async_client: Optional[AsyncFeatherClient] = None

    class Config:
        arbitrary_types_allowed = True

    def __init__(
        self,
        feather_url: str = "http://localhost:8080",
        index_name: str = "llamaindex",
        api_key: Optional[str] = None,
        **kwargs: Any,
    ) -> None:
        super().__init__(
            feather_url=feather_url,
            index_name=index_name,
            api_key=api_key,
            **kwargs,
        )
        self._client = FeatherClient(feather_url, api_key=api_key)
        self._async_client = AsyncFeatherClient(feather_url, api_key=api_key)

    @property
    def client(self) -> FeatherClient:
        """Return the Feather client."""
        if self._client is None:
            self._client = FeatherClient(self.feather_url, api_key=self.api_key)
        return self._client

    @property
    def async_client(self) -> AsyncFeatherClient:
        """Return the async Feather client."""
        if self._async_client is None:
            self._async_client = AsyncFeatherClient(self.feather_url, api_key=self.api_key)
        return self._async_client

    def add(
        self,
        nodes: List[BaseNode],
        **add_kwargs: Any,
    ) -> List[str]:
        """Add nodes to the vector store.

        Args:
            nodes: List of nodes to add
            **add_kwargs: Additional arguments

        Returns:
            List of node IDs
        """
        ids = []
        vectors = {}
        metadata_dict = {}

        for node in nodes:
            node_id = node.node_id or str(uuid.uuid4())
            ids.append(node_id)

            # Get embedding
            embedding = node.get_embedding()
            if embedding is None:
                raise ValueError(f"Node {node_id} has no embedding")

            vectors[node_id] = embedding

            # Prepare metadata
            metadata: Dict[str, Any] = {
                "text": node.get_content(),
                "node_type": node.class_name(),
            }

            # Add node metadata
            if hasattr(node, "metadata") and node.metadata:
                metadata.update(node.metadata)

            # Add ref_doc_id if available
            if hasattr(node, "ref_doc_id") and node.ref_doc_id:
                metadata["ref_doc_id"] = node.ref_doc_id

            metadata_dict[node_id] = metadata

        # Upsert to Feather
        self.client.vectors.upsert(
            index_name=self.index_name,
            vectors=vectors,
            metadata=metadata_dict,
        )

        return ids

    def delete(
        self,
        ref_doc_id: Optional[str] = None,
        node_ids: Optional[List[str]] = None,
        **delete_kwargs: Any,
    ) -> None:
        """Delete nodes from the vector store.

        Args:
            ref_doc_id: Document ID to delete all nodes for
            node_ids: Specific node IDs to delete
            **delete_kwargs: Additional arguments
        """
        if node_ids:
            for node_id in node_ids:
                self.client.vectors.delete(
                    index_name=self.index_name,
                    vector_id=node_id,
                )

        # Note: ref_doc_id filtering would require metadata filtering support
        if ref_doc_id:
            # For now, this is a no-op as Feather doesn't support metadata filtering in delete
            pass

    def query(
        self,
        query: VectorStoreQuery,
        **kwargs: Any,
    ) -> VectorStoreQueryResult:
        """Query the vector store.

        Args:
            query: VectorStoreQuery object
            **kwargs: Additional arguments

        Returns:
            VectorStoreQueryResult with matching nodes
        """
        if query.query_embedding is None:
            raise ValueError("Query must have an embedding")

        # Search in Feather
        results = self.client.vectors.search(
            index_name=self.index_name,
            vector=query.query_embedding,
            top_k=query.similarity_top_k,
        )

        # Convert results to nodes
        nodes = []
        similarities = []
        ids = []

        for result in results:
            metadata = result.metadata or {}
            text = metadata.pop("text", "")
            node_type = metadata.pop("node_type", "TextNode")
            ref_doc_id = metadata.pop("ref_doc_id", None)

            # Create TextNode (most common case)
            node = TextNode(
                id_=result.id,
                text=text,
                metadata=metadata,
            )
            if ref_doc_id:
                node.ref_doc_id = ref_doc_id

            nodes.append(node)
            similarities.append(result.score)
            ids.append(result.id)

        return VectorStoreQueryResult(
            nodes=nodes,
            similarities=similarities,
            ids=ids,
        )


class AsyncFeatherVectorStore(FeatherVectorStore):
    """Async version of FeatherVectorStore for LlamaIndex.

    This class provides async methods for use with async LlamaIndex workflows.

    Example:
        >>> store = AsyncFeatherVectorStore(
        ...     feather_url="http://localhost:8080",
        ...     index_name="docs"
        ... )
        >>> result = await store.aquery(query)
    """

    async def async_add(
        self,
        nodes: List[BaseNode],
        **add_kwargs: Any,
    ) -> List[str]:
        """Async version of add."""
        ids = []
        vectors = {}
        metadata_dict = {}

        for node in nodes:
            node_id = node.node_id or str(uuid.uuid4())
            ids.append(node_id)

            embedding = node.get_embedding()
            if embedding is None:
                raise ValueError(f"Node {node_id} has no embedding")

            vectors[node_id] = embedding

            metadata: Dict[str, Any] = {
                "text": node.get_content(),
                "node_type": node.class_name(),
            }

            if hasattr(node, "metadata") and node.metadata:
                metadata.update(node.metadata)

            if hasattr(node, "ref_doc_id") and node.ref_doc_id:
                metadata["ref_doc_id"] = node.ref_doc_id

            metadata_dict[node_id] = metadata

        await self.async_client.vectors.upsert(
            index_name=self.index_name,
            vectors=vectors,
            metadata=metadata_dict,
        )

        return ids

    async def adelete(
        self,
        ref_doc_id: Optional[str] = None,
        node_ids: Optional[List[str]] = None,
        **delete_kwargs: Any,
    ) -> None:
        """Async version of delete."""
        if node_ids:
            for node_id in node_ids:
                await self.async_client.vectors.delete(
                    index_name=self.index_name,
                    vector_id=node_id,
                )

    async def aquery(
        self,
        query: VectorStoreQuery,
        **kwargs: Any,
    ) -> VectorStoreQueryResult:
        """Async version of query."""
        if query.query_embedding is None:
            raise ValueError("Query must have an embedding")

        results = await self.async_client.vectors.search(
            index_name=self.index_name,
            vector=query.query_embedding,
            top_k=query.similarity_top_k,
        )

        nodes = []
        similarities = []
        ids = []

        for result in results:
            metadata = result.metadata or {}
            text = metadata.pop("text", "")
            node_type = metadata.pop("node_type", "TextNode")
            ref_doc_id = metadata.pop("ref_doc_id", None)

            node = TextNode(
                id_=result.id,
                text=text,
                metadata=metadata,
            )
            if ref_doc_id:
                node.ref_doc_id = ref_doc_id

            nodes.append(node)
            similarities.append(result.score)
            ids.append(result.id)

        return VectorStoreQueryResult(
            nodes=nodes,
            similarities=similarities,
            ids=ids,
        )

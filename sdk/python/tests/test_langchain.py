"""Tests for LangChain integration."""

import pytest
from unittest.mock import Mock, AsyncMock, patch

# Skip if langchain not installed
pytest.importorskip("langchain_core")

from feather_client.integrations.langchain import (
    FeatherVectorStore,
    AsyncFeatherVectorStore,
)


class TestFeatherVectorStore:
    """Tests for the synchronous LangChain vector store."""

    def test_init(self):
        """Test vector store initialization."""
        mock_client = Mock()
        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
            text_key="content",
        )
        assert store._client == mock_client
        assert store._index_name == "test-index"
        assert store._text_key == "content"

    def test_add_texts(self):
        """Test adding texts to the vector store."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.upsert = Mock()

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        texts = ["Hello world", "Goodbye world"]
        metadatas = [{"source": "test1"}, {"source": "test2"}]
        ids = ["id1", "id2"]

        result = store.add_texts(texts, metadatas=metadatas, ids=ids)

        assert result == ids
        mock_client.vectors.upsert.assert_called_once()

    def test_add_texts_generates_ids(self):
        """Test that IDs are generated if not provided."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.upsert = Mock()

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        texts = ["Hello world"]
        result = store.add_texts(texts)

        assert len(result) == 1
        assert result[0]  # Should be a non-empty string

    def test_similarity_search(self):
        """Test similarity search."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.search = Mock(
            return_value=[
                Mock(id="id1", score=0.9, metadata={"text": "Hello", "source": "test"}),
                Mock(id="id2", score=0.8, metadata={"text": "World", "source": "test"}),
            ]
        )

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        # Mock the embedding function
        store._embed_query = Mock(return_value=[0.1, 0.2, 0.3])

        results = store.similarity_search("hello", k=2)

        assert len(results) == 2
        mock_client.vectors.search.assert_called_once()

    def test_similarity_search_with_score(self):
        """Test similarity search with scores."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.search = Mock(
            return_value=[
                Mock(id="id1", score=0.9, metadata={"text": "Hello"}),
            ]
        )

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        store._embed_query = Mock(return_value=[0.1, 0.2, 0.3])

        results = store.similarity_search_with_score("hello", k=1)

        assert len(results) == 1
        doc, score = results[0]
        assert score == 0.9

    def test_delete(self):
        """Test deleting documents."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.delete = Mock(return_value=True)

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        result = store.delete(ids=["id1", "id2"])

        assert result is True
        assert mock_client.vectors.delete.call_count == 2


class TestAsyncFeatherVectorStore:
    """Tests for the async LangChain vector store."""

    @pytest.mark.asyncio
    async def test_init(self):
        """Test async vector store initialization."""
        mock_client = Mock()
        store = AsyncFeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )
        assert store._client == mock_client
        assert store._index_name == "test-index"

    @pytest.mark.asyncio
    async def test_aadd_texts(self):
        """Test async adding texts."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.upsert = AsyncMock()

        store = AsyncFeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        texts = ["Hello world"]
        result = await store.aadd_texts(texts)

        assert len(result) == 1

    @pytest.mark.asyncio
    async def test_asimilarity_search(self):
        """Test async similarity search."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.search = AsyncMock(
            return_value=[
                Mock(id="id1", score=0.9, metadata={"text": "Hello"}),
            ]
        )

        store = AsyncFeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        store._embed_query = AsyncMock(return_value=[0.1, 0.2, 0.3])

        results = await store.asimilarity_search("hello", k=1)

        assert len(results) == 1

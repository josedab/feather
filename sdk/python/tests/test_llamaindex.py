"""Tests for LlamaIndex integration."""

import pytest
from unittest.mock import Mock, patch

# Skip if llama_index not installed
pytest.importorskip("llama_index.core")

from feather_client.integrations.llamaindex import FeatherVectorStore


class TestFeatherVectorStore:
    """Tests for the LlamaIndex vector store."""

    def test_init(self):
        """Test vector store initialization."""
        mock_client = Mock()
        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )
        assert store._client == mock_client
        assert store._index_name == "test-index"

    def test_add_nodes(self):
        """Test adding nodes to the vector store."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.upsert = Mock()

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        # Create mock nodes
        mock_node1 = Mock()
        mock_node1.node_id = "node1"
        mock_node1.get_embedding.return_value = [0.1, 0.2, 0.3]
        mock_node1.get_content.return_value = "Hello world"
        mock_node1.metadata = {"source": "test"}

        mock_node2 = Mock()
        mock_node2.node_id = "node2"
        mock_node2.get_embedding.return_value = [0.4, 0.5, 0.6]
        mock_node2.get_content.return_value = "Goodbye world"
        mock_node2.metadata = {"source": "test"}

        nodes = [mock_node1, mock_node2]
        result = store.add(nodes)

        assert result == ["node1", "node2"]
        mock_client.vectors.upsert.assert_called_once()

    def test_query(self):
        """Test querying the vector store."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.search = Mock(
            return_value=[
                Mock(
                    id="node1",
                    score=0.9,
                    metadata={"text": "Hello world", "_node_content": "Hello world"},
                ),
                Mock(
                    id="node2",
                    score=0.8,
                    metadata={"text": "Hi there", "_node_content": "Hi there"},
                ),
            ]
        )

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        # Create mock query
        mock_query = Mock()
        mock_query.query_embedding = [0.1, 0.2, 0.3]
        mock_query.similarity_top_k = 2
        mock_query.filters = None

        result = store.query(mock_query)

        assert len(result.nodes) == 2
        assert len(result.similarities) == 2
        assert result.similarities[0] == 0.9
        assert result.similarities[1] == 0.8
        mock_client.vectors.search.assert_called_once()

    def test_query_with_filters(self):
        """Test querying with metadata filters."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.search = Mock(
            return_value=[
                Mock(
                    id="node1",
                    score=0.9,
                    metadata={"text": "Hello", "category": "greeting"},
                ),
            ]
        )

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        mock_query = Mock()
        mock_query.query_embedding = [0.1, 0.2, 0.3]
        mock_query.similarity_top_k = 5
        mock_query.filters = Mock()
        mock_query.filters.filters = [
            Mock(key="category", value="greeting", operator="==")
        ]

        result = store.query(mock_query)

        assert len(result.nodes) == 1

    def test_delete_nodes(self):
        """Test deleting nodes."""
        mock_client = Mock()
        mock_client.vectors = Mock()
        mock_client.vectors.delete = Mock()

        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )

        store.delete_nodes(node_ids=["node1", "node2"])

        assert mock_client.vectors.delete.call_count == 2
        mock_client.vectors.delete.assert_any_call("test-index", "node1")
        mock_client.vectors.delete.assert_any_call("test-index", "node2")

    def test_client_property(self):
        """Test the client property."""
        mock_client = Mock()
        store = FeatherVectorStore(
            client=mock_client,
            index_name="test-index",
        )
        assert store.client == mock_client

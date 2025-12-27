"""Synchronous Feather Feature Store client."""

from typing import Any, Optional

import httpx

from feather_client.models import (
    CreateIndexRequest,
    EntityFeatures,
    Feature,
    FeatherError,
    FeatureGroup,
    GetFeaturesResponse,
    HealthStatus,
    NotFoundError,
    SearchVectorsRequest,
    UpsertResponse,
    UpsertVectorsRequest,
    ValidationError,
    VectorIndex,
    VectorRecord,
    VectorSearchResponse,
    VectorSearchResult,
)


class VectorClient:
    """Client for vector similarity search operations."""

    def __init__(self, http: httpx.Client, base_url: str):
        self._http = http
        self._base_url = base_url

    def list_indexes(self) -> list[str]:
        """List all vector indexes."""
        response = self._http.get(f"{self._base_url}/v1/vectors")
        self._check_response(response)
        data = response.json()
        return data.get("indexes", [])

    def create_index(
        self,
        name: str,
        dimension: int,
        distance_type: str = "cosine",
    ) -> VectorIndex:
        """Create a new vector index.

        Args:
            name: Index name
            dimension: Vector dimension
            distance_type: Distance metric (cosine, euclidean, dot_product)

        Returns:
            VectorIndex with the created index info
        """
        request = CreateIndexRequest(
            name=name,
            dimension=dimension,
            distance_type=distance_type,
        )
        response = self._http.post(
            f"{self._base_url}/v1/vectors",
            json=request.model_dump(),
        )
        self._check_response(response)
        return VectorIndex(**response.json())

    def get_index(self, name: str) -> VectorIndex:
        """Get information about a vector index."""
        response = self._http.get(f"{self._base_url}/v1/vectors/{name}")
        self._check_response(response)
        return VectorIndex(**response.json())

    def delete_index(self, name: str) -> None:
        """Delete a vector index."""
        response = self._http.delete(f"{self._base_url}/v1/vectors/{name}")
        self._check_response(response)

    def upsert(
        self,
        index: str,
        vectors: list[dict[str, Any]],
    ) -> int:
        """Upsert vectors into an index.

        Args:
            index: Index name
            vectors: List of dicts with id, vector, and optional metadata

        Returns:
            Number of vectors upserted
        """
        records = [
            VectorRecord(
                id=v["id"],
                vector=v["vector"],
                metadata=v.get("metadata"),
            )
            for v in vectors
        ]
        request = UpsertVectorsRequest(vectors=records)
        response = self._http.post(
            f"{self._base_url}/v1/vectors/{index}/upsert",
            json=request.model_dump(exclude_none=True),
        )
        self._check_response(response)
        result = UpsertResponse(**response.json())
        return result.upserted

    def search(
        self,
        index: str,
        vector: list[float],
        top_k: int = 10,
        *,
        ef: Optional[int] = None,
        include_metadata: bool = True,
        include_vectors: bool = False,
    ) -> list[VectorSearchResult]:
        """Search for similar vectors.

        Args:
            index: Index name
            vector: Query vector
            top_k: Number of results to return
            ef: Search expansion factor (higher = more accurate)
            include_metadata: Include metadata in results
            include_vectors: Include vectors in results

        Returns:
            List of search results
        """
        request = SearchVectorsRequest(
            vector=vector,
            top_k=top_k,
            ef=ef,
            include_metadata=include_metadata,
            include_vectors=include_vectors,
        )
        response = self._http.post(
            f"{self._base_url}/v1/vectors/{index}/search",
            json=request.model_dump(exclude_none=True),
        )
        self._check_response(response)
        result = VectorSearchResponse(**response.json())
        return result.results

    def get(self, index: str, vector_id: str, include_vector: bool = False) -> VectorRecord:
        """Get a vector by ID."""
        params = {"include_vector": "true"} if include_vector else {}
        response = self._http.get(
            f"{self._base_url}/v1/vectors/{index}/{vector_id}",
            params=params,
        )
        self._check_response(response)
        return VectorRecord(**response.json())

    def delete(self, index: str, vector_id: str) -> None:
        """Delete a vector by ID."""
        response = self._http.delete(f"{self._base_url}/v1/vectors/{index}/{vector_id}")
        self._check_response(response)

    def _check_response(self, response: httpx.Response) -> None:
        if response.status_code == 404:
            data = response.json()
            raise NotFoundError(data.get("error", "Not found"), response.status_code)
        elif response.status_code == 400:
            data = response.json()
            raise ValidationError(data.get("error", "Validation error"), response.status_code)
        elif response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise FeatherError(message, response.status_code)


class FeatherClient:
    """Synchronous client for Feather Feature Store.

    Example:
        >>> client = FeatherClient("http://localhost:8080")
        >>> features = client.get_features("user:123", ["purchase_count"])
        >>> print(features["purchase_count"].value)
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        *,
        timeout: float = 30.0,
        headers: Optional[dict[str, str]] = None,
    ):
        """Initialize the Feather client.

        Args:
            base_url: Base URL of the Feather server
            timeout: Request timeout in seconds
            headers: Additional headers to send with requests
        """
        self._base_url = base_url.rstrip("/")
        self._http = httpx.Client(
            timeout=timeout,
            headers=headers or {},
        )
        self._vectors = VectorClient(self._http, self._base_url)

    @property
    def vectors(self) -> VectorClient:
        """Access vector operations."""
        return self._vectors

    def close(self) -> None:
        """Close the client connection."""
        self._http.close()

    def __enter__(self) -> "FeatherClient":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()

    def get_features(
        self,
        entity: str,
        features: list[str],
    ) -> dict[str, Feature]:
        """Get features for an entity.

        Args:
            entity: Entity key (e.g., "user:123")
            features: List of feature names to retrieve

        Returns:
            Dictionary mapping feature names to Feature objects
        """
        params = {"entity": entity, "feature": features}
        response = self._http.get(f"{self._base_url}/v1/features", params=params)
        self._check_response(response)

        data = GetFeaturesResponse(**response.json())
        if entity not in data.entities:
            return {}
        return {
            name: feature
            for name, feature in data.entities[entity].features.items()
        }

    def get_features_batch(
        self,
        entities: list[str],
        features: list[str],
    ) -> dict[str, dict[str, Feature]]:
        """Get features for multiple entities.

        Args:
            entities: List of entity keys
            features: List of feature names to retrieve

        Returns:
            Nested dictionary mapping entity -> feature_name -> Feature
        """
        response = self._http.post(
            f"{self._base_url}/v1/features/batch",
            json={"entities": entities, "features": features},
        )
        self._check_response(response)

        data = GetFeaturesResponse(**response.json())
        return {
            entity_key: {
                name: feature
                for name, feature in entity.features.items()
            }
            for entity_key, entity in data.entities.items()
        }

    def put_features(
        self,
        entity: str,
        features: dict[str, Any],
        *,
        timestamp: Optional[int] = None,
        version: Optional[int] = None,
    ) -> None:
        """Store features for an entity.

        Args:
            entity: Entity key
            features: Dictionary of feature name to value
            timestamp: Optional timestamp (nanoseconds since epoch)
            version: Optional version number
        """
        payload: dict[str, Any] = {
            "entity_key": entity,
            "features": features,
        }
        if timestamp is not None:
            payload["timestamp"] = timestamp
        if version is not None:
            payload["version"] = version

        response = self._http.post(f"{self._base_url}/v1/features", json=payload)
        self._check_response(response)

    def put_features_batch(
        self,
        updates: list[dict[str, Any]],
        *,
        ingestion_url: Optional[str] = None,
    ) -> dict[str, int]:
        """Store features for multiple entities in a single batch request.

        This method uses the ingestion bulk endpoint for efficient batch updates,
        avoiding the N+1 problem of calling put_features individually.

        Args:
            updates: List of feature updates, each containing:
                - entity_key: Entity key (required)
                - features: Dictionary of feature name to value (required)
                - timestamp: Optional timestamp (nanoseconds since epoch)
                - version: Optional version number
            ingestion_url: Optional URL for ingestion server (defaults to port 8081)

        Returns:
            Dictionary with 'success', 'errors', and 'total' counts

        Example:
            >>> client.put_features_batch([
            ...     {"entity_key": "user:1", "features": {"score": 0.9}},
            ...     {"entity_key": "user:2", "features": {"score": 0.8}},
            ... ])
        """
        if not updates:
            return {"success": 0, "errors": 0, "total": 0}

        # Derive ingestion URL from base_url if not provided
        if ingestion_url is None:
            # Replace port with 8081 for ingestion server
            from urllib.parse import urlparse, urlunparse

            parsed = urlparse(self._base_url)
            # Default to port 8081 for ingestion
            ingestion_url = urlunparse(
                (parsed.scheme, f"{parsed.hostname}:8081", "", "", "", "")
            )

        response = self._http.post(
            f"{ingestion_url}/ingest/bulk",
            json=updates,
        )
        self._check_response(response)
        return response.json()

    def get_features_as_of(
        self,
        entity: str,
        features: list[str],
        as_of: str,
    ) -> dict[str, Feature]:
        """Get features as of a specific time (point-in-time retrieval).

        Args:
            entity: Entity key
            features: List of feature names
            as_of: ISO 8601 timestamp (e.g., "2024-01-15T10:30:00Z")

        Returns:
            Dictionary mapping feature names to Feature objects
        """
        params = {"entity": entity, "feature": features, "as_of": as_of}
        response = self._http.get(f"{self._base_url}/v1/features/history", params=params)
        self._check_response(response)

        data = GetFeaturesResponse(**response.json())
        if entity not in data.entities:
            return {}
        return {
            name: feature
            for name, feature in data.entities[entity].features.items()
        }

    def list_groups(self) -> list[FeatureGroup]:
        """List all feature groups."""
        response = self._http.get(f"{self._base_url}/v1/schema/groups")
        self._check_response(response)
        return [FeatureGroup(**g) for g in response.json()]

    def get_group(self, name: str) -> FeatureGroup:
        """Get a feature group by name."""
        response = self._http.get(f"{self._base_url}/v1/schema/groups/{name}")
        self._check_response(response)
        return FeatureGroup(**response.json())

    def create_group(self, group: FeatureGroup) -> FeatureGroup:
        """Create a new feature group."""
        response = self._http.post(
            f"{self._base_url}/v1/schema/groups",
            json=group.model_dump(exclude_none=True),
        )
        self._check_response(response)
        return FeatureGroup(**response.json())

    def health(self) -> HealthStatus:
        """Check server health."""
        response = self._http.get(f"{self._base_url}/health")
        return HealthStatus(**response.json())

    def ready(self) -> bool:
        """Check if server is ready."""
        response = self._http.get(f"{self._base_url}/ready")
        return response.status_code == 200

    def live(self) -> bool:
        """Check if server is alive."""
        response = self._http.get(f"{self._base_url}/live")
        return response.status_code == 200

    def _check_response(self, response: httpx.Response) -> None:
        if response.status_code == 404:
            data = response.json()
            raise NotFoundError(data.get("error", "Not found"), response.status_code)
        elif response.status_code == 400:
            data = response.json()
            raise ValidationError(data.get("error", "Validation error"), response.status_code)
        elif response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise FeatherError(message, response.status_code)

"""Asynchronous Feather Feature Store client with connection pooling and retry."""

import asyncio
import random
from typing import Any, Callable, Optional, TypeVar

import httpx

from feather_client.models import (
    BenchmarkRequest,
    BenchmarkResult,
    CreateIndexRequest,
    Feature,
    FeatherError,
    FeatureGroup,
    GetFeaturesResponse,
    HealthStatus,
    MLConnector,
    NotFoundError,
    PredictRequest,
    PredictResponse,
    SearchVectorsRequest,
    Transform,
    TransformExecuteRequest,
    UpsertResponse,
    UpsertVectorsRequest,
    ValidationError,
    VectorIndex,
    VectorRecord,
    VectorSearchResponse,
    VectorSearchResult,
)

T = TypeVar("T")


class AsyncVectorClient:
    """Async client for vector similarity search operations."""

    def __init__(
        self,
        http: httpx.AsyncClient,
        base_url: str,
        max_retries: int = 0,
        retry_delay: float = 0.1,
    ):
        self._http = http
        self._base_url = base_url
        self._max_retries = max_retries
        self._retry_delay = retry_delay

    async def _request_with_retry(self, fn: Callable[[], Any]) -> Any:
        """Execute a request with retry if retries are configured."""
        if self._max_retries > 0:
            return await with_retry(
                fn,
                max_retries=self._max_retries,
                base_delay=self._retry_delay,
            )
        return await fn()

    async def list_indexes(self) -> list[str]:
        """List all vector indexes."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/vectors")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = response.json()
        return data.get("indexes", [])

    async def create_index(
        self,
        name: str,
        dimension: int,
        distance_type: str = "cosine",
    ) -> VectorIndex:
        """Create a new vector index."""
        request = CreateIndexRequest(
            name=name,
            dimension=dimension,
            distance_type=distance_type,
        )

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/vectors",
                json=request.model_dump(),
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return VectorIndex(**response.json())

    async def get_index(self, name: str) -> VectorIndex:
        """Get information about a vector index."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/vectors/{name}")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return VectorIndex(**response.json())

    async def delete_index(self, name: str) -> None:
        """Delete a vector index."""

        async def _do_request() -> httpx.Response:
            response = await self._http.delete(f"{self._base_url}/v1/vectors/{name}")
            self._check_response(response)
            return response

        await self._request_with_retry(_do_request)

    async def upsert(
        self,
        index: str,
        vectors: list[dict[str, Any]],
    ) -> int:
        """Upsert vectors into an index."""
        records = [
            VectorRecord(
                id=v["id"],
                vector=v["vector"],
                metadata=v.get("metadata"),
            )
            for v in vectors
        ]
        request = UpsertVectorsRequest(vectors=records)

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/vectors/{index}/upsert",
                json=request.model_dump(exclude_none=True),
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        result = UpsertResponse(**response.json())
        return result.upserted

    async def search(
        self,
        index: str,
        vector: list[float],
        top_k: int = 10,
        *,
        ef: Optional[int] = None,
        include_metadata: bool = True,
        include_vectors: bool = False,
    ) -> list[VectorSearchResult]:
        """Search for similar vectors."""
        request = SearchVectorsRequest(
            vector=vector,
            top_k=top_k,
            ef=ef,
            include_metadata=include_metadata,
            include_vectors=include_vectors,
        )

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/vectors/{index}/search",
                json=request.model_dump(exclude_none=True),
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        result = VectorSearchResponse(**response.json())
        return result.results

    async def get(
        self, index: str, vector_id: str, include_vector: bool = False
    ) -> VectorRecord:
        """Get a vector by ID."""
        params = {"include_vector": "true"} if include_vector else {}

        async def _do_request() -> httpx.Response:
            response = await self._http.get(
                f"{self._base_url}/v1/vectors/{index}/{vector_id}",
                params=params,
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return VectorRecord(**response.json())

    async def delete(self, index: str, vector_id: str) -> None:
        """Delete a vector by ID."""

        async def _do_request() -> httpx.Response:
            response = await self._http.delete(
                f"{self._base_url}/v1/vectors/{index}/{vector_id}"
            )
            self._check_response(response)
            return response

        await self._request_with_retry(_do_request)

    def _check_response(self, response: httpx.Response) -> None:
        if response.status_code == 404:
            data = response.json()
            raise NotFoundError(data.get("error", "Not found"), response.status_code)
        elif response.status_code == 400:
            data = response.json()
            raise ValidationError(
                data.get("error", "Validation error"), response.status_code
            )
        elif response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise FeatherError(message, response.status_code)


class AsyncTransformClient:
    """Async client for feature transformation operations."""

    def __init__(
        self,
        http: httpx.AsyncClient,
        base_url: str,
        max_retries: int = 0,
        retry_delay: float = 0.1,
    ):
        self._http = http
        self._base_url = base_url
        self._max_retries = max_retries
        self._retry_delay = retry_delay

    async def _request_with_retry(self, fn: Callable[[], Any]) -> Any:
        """Execute a request with retry if retries are configured."""
        if self._max_retries > 0:
            return await with_retry(
                fn,
                max_retries=self._max_retries,
                base_delay=self._retry_delay,
            )
        return await fn()

    async def list(self) -> list[Transform]:
        """List all transforms."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/transforms")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = response.json()
        return [Transform(**t) for t in data.get("transforms", [])]

    async def get(self, name: str) -> Transform:
        """Get a transform by name."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/transforms/{name}")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return Transform(**response.json())

    async def create(self, transform: Transform) -> Transform:
        """Create a new transform."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/transforms",
                json=transform.model_dump(exclude_none=True),
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return Transform(**response.json())

    async def delete(self, name: str) -> None:
        """Delete a transform."""

        async def _do_request() -> httpx.Response:
            response = await self._http.delete(f"{self._base_url}/v1/transforms/{name}")
            self._check_response(response)
            return response

        await self._request_with_retry(_do_request)

    async def define_dsl(self, name: str, expression: str) -> Transform:
        """Define a transform using DSL."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/transforms/dsl",
                json={"name": name, "expression": expression},
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return Transform(**response.json())

    async def execute(self, name: str, entity_id: str) -> Any:
        """Execute a transform for an entity."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/transforms/{name}/execute",
                json={"entity_id": entity_id},
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = response.json()
        return data.get("result")

    async def execute_and_store(self, name: str, entity_id: str) -> str:
        """Execute a transform and store the result."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/transforms/{name}/execute-store",
                json={"entity_id": entity_id},
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = response.json()
        return data.get("output_feature", "")

    async def execute_chain(self, output_feature: str, entity_id: str) -> Any:
        """Execute a chain of dependent transforms."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/transforms/chain",
                json={"output_feature": output_feature, "entity_id": entity_id},
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = response.json()
        return data.get("result")

    def _check_response(self, response: httpx.Response) -> None:
        if response.status_code == 404:
            data = response.json()
            raise NotFoundError(data.get("error", "Not found"), response.status_code)
        elif response.status_code == 400:
            data = response.json()
            raise ValidationError(
                data.get("error", "Validation error"), response.status_code
            )
        elif response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise FeatherError(message, response.status_code)


class AsyncMLClient:
    """Async client for ML connector operations."""

    def __init__(
        self,
        http: httpx.AsyncClient,
        base_url: str,
        max_retries: int = 0,
        retry_delay: float = 0.1,
    ):
        self._http = http
        self._base_url = base_url
        self._max_retries = max_retries
        self._retry_delay = retry_delay

    async def _request_with_retry(self, fn: Callable[[], Any]) -> Any:
        """Execute a request with retry if retries are configured."""
        if self._max_retries > 0:
            return await with_retry(
                fn,
                max_retries=self._max_retries,
                base_delay=self._retry_delay,
            )
        return await fn()

    async def list_connectors(self) -> list[MLConnector]:
        """List all ML connectors."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/ml/connectors")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = response.json()
        return [MLConnector(**c) for c in data.get("connectors", [])]

    async def register_connector(
        self,
        name: str,
        connector_type: str,
        endpoint: str,
        **kwargs: Any,
    ) -> MLConnector:
        """Register a new ML connector."""
        payload = {
            "name": name,
            "type": connector_type,
            "endpoint": endpoint,
            **kwargs,
        }

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/ml/connectors",
                json=payload,
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return MLConnector(**response.json())

    async def get_connector(self, name: str) -> MLConnector:
        """Get a connector by name."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/ml/connectors/{name}")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return MLConnector(**response.json())

    async def unregister_connector(self, name: str) -> None:
        """Unregister a connector."""

        async def _do_request() -> httpx.Response:
            response = await self._http.delete(f"{self._base_url}/v1/ml/connectors/{name}")
            self._check_response(response)
            return response

        await self._request_with_retry(_do_request)

    async def connect(self, name: str) -> None:
        """Connect a connector."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/ml/connectors/{name}/connect"
            )
            self._check_response(response)
            return response

        await self._request_with_retry(_do_request)

    async def disconnect(self, name: str) -> None:
        """Disconnect a connector."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/ml/connectors/{name}/disconnect"
            )
            self._check_response(response)
            return response

        await self._request_with_retry(_do_request)

    async def predict(
        self,
        connector: str,
        model_name: str,
        *,
        entity_id: Optional[str] = None,
        feature_names: Optional[list[str]] = None,
        features: Optional[dict[str, Any]] = None,
        model_version: Optional[str] = None,
    ) -> PredictResponse:
        """Make a prediction."""
        payload: dict[str, Any] = {
            "connector": connector,
            "model_name": model_name,
        }
        if entity_id:
            payload["entity_id"] = entity_id
        if feature_names:
            payload["feature_names"] = feature_names
        if features:
            payload["features"] = features
        if model_version:
            payload["model_version"] = model_version

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/ml/predict",
                json=payload,
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return PredictResponse(**response.json())

    async def batch_predict(
        self,
        connector: str,
        model_name: str,
        *,
        entity_ids: Optional[list[str]] = None,
        feature_names: Optional[list[str]] = None,
        features: Optional[list[dict[str, Any]]] = None,
        model_version: Optional[str] = None,
    ) -> PredictResponse:
        """Make batch predictions."""
        payload: dict[str, Any] = {
            "connector": connector,
            "model_name": model_name,
        }
        if entity_ids:
            payload["entity_ids"] = entity_ids
        if feature_names:
            payload["feature_names"] = feature_names
        if features:
            payload["features"] = features
        if model_version:
            payload["model_version"] = model_version

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/ml/predict/batch",
                json=payload,
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return PredictResponse(**response.json())

    def _check_response(self, response: httpx.Response) -> None:
        if response.status_code == 404:
            data = response.json()
            raise NotFoundError(data.get("error", "Not found"), response.status_code)
        elif response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise FeatherError(message, response.status_code)


class AsyncBenchmarkClient:
    """Async client for benchmark operations."""

    def __init__(
        self,
        http: httpx.AsyncClient,
        base_url: str,
        max_retries: int = 0,
        retry_delay: float = 0.1,
    ):
        self._http = http
        self._base_url = base_url
        self._max_retries = max_retries
        self._retry_delay = retry_delay

    async def _request_with_retry(self, fn: Callable[[], Any]) -> Any:
        """Execute a request with retry if retries are configured."""
        if self._max_retries > 0:
            return await with_retry(
                fn,
                max_retries=self._max_retries,
                base_delay=self._retry_delay,
            )
        return await fn()

    async def run(
        self,
        benchmark_type: str,
        iterations: int = 1000,
        concurrency: int = 1,
        value_size: int = 100,
    ) -> BenchmarkResult:
        """Run a benchmark."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/benchmarks/run",
                json={
                    "type": benchmark_type,
                    "iterations": iterations,
                    "concurrency": concurrency,
                    "value_size": value_size,
                },
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return BenchmarkResult(**response.json())

    async def run_suite(
        self,
        iterations: int = 1000,
        concurrency: int = 1,
    ) -> list[BenchmarkResult]:
        """Run all benchmarks."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/benchmarks/suite",
                json={
                    "iterations": iterations,
                    "concurrency": concurrency,
                },
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = response.json()
        return [BenchmarkResult(**r) for r in data.get("results", [])]

    def _check_response(self, response: httpx.Response) -> None:
        if response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise FeatherError(message, response.status_code)


class RetryableHTTPError(Exception):
    """Exception for HTTP errors that should be retried (5xx)."""

    def __init__(self, status_code: int, message: str):
        self.status_code = status_code
        super().__init__(message)


async def with_retry(
    fn: Callable[[], Any],
    max_retries: int = 3,
    base_delay: float = 0.1,
    max_delay: float = 10.0,
    exponential_base: float = 2.0,
    jitter: float = 0.1,
    retryable_exceptions: tuple[type[Exception], ...] = (
        httpx.ConnectError,
        httpx.ReadTimeout,
        httpx.WriteTimeout,
        RetryableHTTPError,
    ),
) -> Any:
    """Execute a function with exponential backoff retry and jitter.

    Args:
        fn: Async function to execute
        max_retries: Maximum number of retry attempts
        base_delay: Initial delay between retries in seconds
        max_delay: Maximum delay between retries in seconds
        exponential_base: Base for exponential backoff calculation
        jitter: Random jitter factor (0.1 = ±10% randomness)
        retryable_exceptions: Tuple of exception types that trigger retry
    """
    last_exception: Optional[Exception] = None
    delay = base_delay

    for attempt in range(max_retries + 1):
        try:
            return await fn()
        except retryable_exceptions as e:
            last_exception = e
            if attempt == max_retries:
                break
            # Apply jitter: delay * (1 + random(-jitter, +jitter))
            jittered_delay = delay * (1 + random.uniform(-jitter, jitter))
            await asyncio.sleep(min(jittered_delay, max_delay))
            delay *= exponential_base

    raise last_exception if last_exception else RuntimeError("Retry failed")


class AsyncFeatherClient:
    """Asynchronous client for Feather Feature Store with connection pooling and retry.

    Example:
        >>> async with AsyncFeatherClient("http://localhost:8080") as client:
        ...     features = await client.get_features("user:123", ["purchase_count"])
        ...     print(features["purchase_count"].value)

    Example with retry:
        >>> client = AsyncFeatherClient(
        ...     "http://localhost:8080",
        ...     max_retries=3,
        ...     retry_delay=0.1,
        ... )
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        *,
        timeout: float = 30.0,
        headers: Optional[dict[str, str]] = None,
        max_connections: int = 100,
        max_keepalive_connections: int = 20,
        keepalive_expiry: float = 5.0,
        max_retries: int = 0,
        retry_delay: float = 0.1,
    ):
        """Initialize the async Feather client.

        Args:
            base_url: Base URL of the Feather server
            timeout: Request timeout in seconds
            headers: Additional headers to send with requests
            max_connections: Maximum number of connections in the pool
            max_keepalive_connections: Maximum number of idle connections
            keepalive_expiry: Time to keep idle connections alive (seconds)
            max_retries: Maximum number of retries for failed requests (0 = no retry)
            retry_delay: Base delay between retries (exponential backoff)
        """
        self._base_url = base_url.rstrip("/")
        self._max_retries = max_retries
        self._retry_delay = retry_delay

        # Connection pooling settings
        limits = httpx.Limits(
            max_connections=max_connections,
            max_keepalive_connections=max_keepalive_connections,
            keepalive_expiry=keepalive_expiry,
        )

        self._http = httpx.AsyncClient(
            timeout=timeout,
            headers=headers or {},
            limits=limits,
        )

        # Initialize sub-clients with retry configuration
        self._vectors = AsyncVectorClient(
            self._http, self._base_url, max_retries, retry_delay
        )
        self._transforms = AsyncTransformClient(
            self._http, self._base_url, max_retries, retry_delay
        )
        self._ml = AsyncMLClient(
            self._http, self._base_url, max_retries, retry_delay
        )
        self._benchmarks = AsyncBenchmarkClient(
            self._http, self._base_url, max_retries, retry_delay
        )

    @property
    def vectors(self) -> AsyncVectorClient:
        """Access vector operations."""
        return self._vectors

    @property
    def transforms(self) -> AsyncTransformClient:
        """Access transform operations."""
        return self._transforms

    @property
    def ml(self) -> AsyncMLClient:
        """Access ML connector operations."""
        return self._ml

    @property
    def benchmarks(self) -> AsyncBenchmarkClient:
        """Access benchmark operations."""
        return self._benchmarks

    async def close(self) -> None:
        """Close the client connection."""
        await self._http.aclose()

    async def __aenter__(self) -> "AsyncFeatherClient":
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()

    async def get_features(
        self,
        entity: str,
        features: list[str],
    ) -> dict[str, Feature]:
        """Get features for an entity."""
        params = {"entity": entity, "feature": features}

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/features", params=params)
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = GetFeaturesResponse(**response.json())
        if entity not in data.entities:
            return {}
        return {
            name: feature for name, feature in data.entities[entity].features.items()
        }

    async def get_features_batch(
        self,
        entities: list[str],
        features: list[str],
    ) -> dict[str, dict[str, Feature]]:
        """Get features for multiple entities."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/features/batch",
                json={"entities": entities, "features": features},
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = GetFeaturesResponse(**response.json())
        return {
            entity_key: {name: feature for name, feature in entity.features.items()}
            for entity_key, entity in data.entities.items()
        }

    async def put_features(
        self,
        entity: str,
        features: dict[str, Any],
        *,
        timestamp: Optional[int] = None,
        version: Optional[int] = None,
    ) -> None:
        """Store features for an entity."""
        payload: dict[str, Any] = {
            "entity_key": entity,
            "features": features,
        }
        if timestamp is not None:
            payload["timestamp"] = timestamp
        if version is not None:
            payload["version"] = version

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/features", json=payload
            )
            self._check_response(response)
            return response

        await self._request_with_retry(_do_request)

    async def get_features_as_of(
        self,
        entity: str,
        features: list[str],
        as_of: str,
    ) -> dict[str, Feature]:
        """Get features as of a specific time."""
        params = {"entity": entity, "feature": features, "as_of": as_of}

        async def _do_request() -> httpx.Response:
            response = await self._http.get(
                f"{self._base_url}/v1/features/history", params=params
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        data = GetFeaturesResponse(**response.json())
        if entity not in data.entities:
            return {}
        return {
            name: feature for name, feature in data.entities[entity].features.items()
        }

    async def list_groups(self) -> list[FeatureGroup]:
        """List all feature groups."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/schema/groups")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return [FeatureGroup(**g) for g in response.json()]

    async def get_group(self, name: str) -> FeatureGroup:
        """Get a feature group by name."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/v1/schema/groups/{name}")
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return FeatureGroup(**response.json())

    async def create_group(self, group: FeatureGroup) -> FeatureGroup:
        """Create a new feature group."""

        async def _do_request() -> httpx.Response:
            response = await self._http.post(
                f"{self._base_url}/v1/schema/groups",
                json=group.model_dump(exclude_none=True),
            )
            self._check_response(response)
            return response

        response = await self._request_with_retry(_do_request)
        return FeatureGroup(**response.json())

    async def health(self) -> HealthStatus:
        """Check server health."""

        async def _do_request() -> httpx.Response:
            response = await self._http.get(f"{self._base_url}/health")
            return response

        response = await self._request_with_retry(_do_request)
        return HealthStatus(**response.json())

    async def ready(self) -> bool:
        """Check if server is ready."""
        response = await self._http.get(f"{self._base_url}/ready")
        return response.status_code == 200

    async def live(self) -> bool:
        """Check if server is alive."""
        response = await self._http.get(f"{self._base_url}/live")
        return response.status_code == 200

    async def _request_with_retry(self, fn: Callable[[], Any]) -> Any:
        """Execute a request with retry if retries are configured."""
        if self._max_retries > 0:
            return await with_retry(
                fn,
                max_retries=self._max_retries,
                base_delay=self._retry_delay,
            )
        return await fn()

    def _check_response(self, response: httpx.Response) -> None:
        """Check HTTP response and raise appropriate exceptions."""
        if response.status_code == 404:
            data = response.json()
            raise NotFoundError(data.get("error", "Not found"), response.status_code)
        elif response.status_code == 400:
            data = response.json()
            raise ValidationError(
                data.get("error", "Validation error"), response.status_code
            )
        elif response.status_code >= 500:
            # Server errors should be retried
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise RetryableHTTPError(response.status_code, message)
        elif response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error", response.text)
            except Exception:
                message = response.text
            raise FeatherError(message, response.status_code)

"""Pydantic models for Feather Feature Store API."""

from datetime import datetime
from typing import Any, Optional

from pydantic import BaseModel, Field


class FeatherError(Exception):
    """Base exception for Feather client errors."""

    def __init__(self, message: str, status_code: Optional[int] = None):
        self.message = message
        self.status_code = status_code
        super().__init__(message)


class NotFoundError(FeatherError):
    """Raised when a resource is not found."""

    pass


class ValidationError(FeatherError):
    """Raised when request validation fails."""

    pass


class Feature(BaseModel):
    """A feature with its value and metadata."""

    value: Any
    timestamp: Optional[int] = None

    @property
    def timestamp_datetime(self) -> Optional[datetime]:
        """Convert timestamp to datetime."""
        if self.timestamp:
            return datetime.fromtimestamp(self.timestamp / 1e9)
        return None


class FeatureValue(BaseModel):
    """A feature value for storage."""

    value: Any
    timestamp: Optional[int] = None
    version: Optional[int] = None


class EntityFeatures(BaseModel):
    """Features for a single entity."""

    features: dict[str, Feature]


class GetFeaturesResponse(BaseModel):
    """Response from get features endpoint."""

    entities: dict[str, EntityFeatures]


class FeatureSpec(BaseModel):
    """Specification for a feature within a group."""

    name: str
    data_type: str
    dimensions: Optional[list[int]] = None
    default: Optional[Any] = None


class FeatureGroup(BaseModel):
    """A feature group definition."""

    name: str
    entity_type: str
    ttl: Optional[int] = None
    description: Optional[str] = None
    features: list[FeatureSpec] = Field(default_factory=list)


class VectorIndex(BaseModel):
    """Information about a vector index."""

    name: str
    dimension: int
    distance_type: str = "cosine"
    size: int = 0


class VectorRecord(BaseModel):
    """A vector with its metadata."""

    id: str
    vector: Optional[list[float]] = None
    metadata: Optional[dict[str, Any]] = None


class VectorSearchResult(BaseModel):
    """A result from vector search."""

    id: str
    distance: float
    score: float
    vector: Optional[list[float]] = None
    metadata: Optional[dict[str, Any]] = None


class VectorSearchResponse(BaseModel):
    """Response from vector search endpoint."""

    results: list[VectorSearchResult]


class UpsertResponse(BaseModel):
    """Response from vector upsert endpoint."""

    upserted: int


class HealthStatus(BaseModel):
    """Health check response."""

    status: str
    components: Optional[dict[str, Any]] = None


class CreateIndexRequest(BaseModel):
    """Request to create a vector index."""

    name: str
    dimension: int
    distance_type: str = "cosine"


class UpsertVectorsRequest(BaseModel):
    """Request to upsert vectors."""

    vectors: list[VectorRecord]


class SearchVectorsRequest(BaseModel):
    """Request to search vectors."""

    vector: list[float]
    top_k: int = 10
    ef: Optional[int] = None
    include_metadata: bool = True
    include_vectors: bool = False


# Transform models


class Transform(BaseModel):
    """A feature transformation definition."""

    name: str
    description: Optional[str] = None
    type: str
    expression: Optional[str] = None
    inputs: list[str] = Field(default_factory=list)
    output: str
    output_type: Optional[str] = None
    config: Optional[dict[str, Any]] = None
    enabled: bool = True
    mode: str = "on_read"
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None


class TransformExecuteRequest(BaseModel):
    """Request to execute a transform."""

    entity_id: str


class TransformChainRequest(BaseModel):
    """Request to execute a transform chain."""

    output_feature: str
    entity_id: str


# ML Connector models


class MLConnector(BaseModel):
    """An ML connector definition."""

    name: str
    type: str
    connected: bool = False
    endpoint: Optional[str] = None


class PredictRequest(BaseModel):
    """Request for ML prediction."""

    connector: str
    model_name: str
    model_version: Optional[str] = None
    entity_id: Optional[str] = None
    feature_names: Optional[list[str]] = None
    features: Optional[dict[str, Any]] = None


class PredictResponse(BaseModel):
    """Response from ML prediction."""

    model_name: str
    model_version: Optional[str] = None
    predictions: Any = None
    latency_ms: Optional[int] = None
    count: Optional[int] = None


# Benchmark models


class BenchmarkRequest(BaseModel):
    """Request to run a benchmark."""

    type: str
    iterations: int = 1000
    concurrency: int = 1
    value_size: int = 100


class LatencyStats(BaseModel):
    """Latency statistics."""

    p50_us: float = Field(alias="p50_us")
    p95_us: float = Field(alias="p95_us")
    p99_us: float = Field(alias="p99_us")
    p999_us: float = Field(alias="p999_us")
    mean_us: float = Field(alias="mean_us")
    min_us: float = Field(alias="min_us")
    max_us: float = Field(alias="max_us")

    class Config:
        populate_by_name = True


class BenchmarkResult(BaseModel):
    """Result from a benchmark run."""

    name: str
    iterations: int
    concurrency: int
    duration_ms: float
    ops_per_second: float
    latency: Optional[LatencyStats] = None
    success_rate: float = 1.0
    errors: int = 0


# Observability models


class FeatureMetrics(BaseModel):
    """Metrics for a feature."""

    name: str
    read_count: int = 0
    write_count: int = 0
    cache_hit_rate: float = 0.0
    avg_latency_us: float = 0.0
    p99_latency_us: float = 0.0


class DriftAlert(BaseModel):
    """A drift detection alert."""

    feature: str
    drift_score: float
    threshold: float
    detected_at: datetime
    severity: str = "warning"
    details: Optional[dict[str, Any]] = None


class ConsistencyCheckResult(BaseModel):
    """Result of online/offline consistency check."""

    feature: str
    entity_id: str
    online_value: Any
    offline_value: Any
    is_consistent: bool
    difference: Optional[float] = None
    checked_at: datetime

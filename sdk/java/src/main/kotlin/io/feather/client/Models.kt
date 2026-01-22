package io.feather.client

import com.fasterxml.jackson.annotation.JsonProperty
import java.time.Instant

/**
 * Feature value representation.
 */
typealias FeatureValue = Any?

/**
 * Feature specification in a feature group.
 */
data class FeatureSpec(
    val name: String,
    @JsonProperty("data_type") val dataType: DataType,
    @JsonProperty("default_value") val defaultValue: FeatureValue? = null
)

/**
 * Supported data types for features.
 */
enum class DataType {
    @JsonProperty("string") STRING,
    @JsonProperty("int64") INT64,
    @JsonProperty("float64") FLOAT64,
    @JsonProperty("bool") BOOL,
    @JsonProperty("bytes") BYTES,
    @JsonProperty("timestamp") TIMESTAMP,
    @JsonProperty("string_list") STRING_LIST,
    @JsonProperty("int64_list") INT64_LIST,
    @JsonProperty("float64_list") FLOAT64_LIST,
    @JsonProperty("map") MAP
}

/**
 * Feature group definition.
 */
data class FeatureGroup(
    val name: String,
    val description: String? = null,
    @JsonProperty("entity_type") val entityType: String,
    val features: List<FeatureSpec>,
    val ttl: Long? = null,
    val tags: Map<String, String>? = null
)

/**
 * Response from getting features.
 */
data class GetFeaturesResponse(
    @JsonProperty("entity_id") val entityId: String,
    val features: Map<String, FeatureValue>,
    val metadata: Map<String, Any?>? = null
)

/**
 * Features for an entity in batch operations.
 */
data class EntityFeatures(
    @JsonProperty("entity_id") val entityId: String,
    val features: Map<String, FeatureValue>,
    val timestamp: Instant? = null
)

/**
 * Response from batch get.
 */
data class BatchGetResponse(
    val results: List<EntityFeatures>,
    val errors: Map<String, String>? = null
)

/**
 * Vector index information.
 */
data class VectorIndex(
    val name: String,
    val dimension: Int,
    @JsonProperty("distance_type") val distanceType: DistanceType,
    @JsonProperty("vector_count") val vectorCount: Long,
    @JsonProperty("created_at") val createdAt: Instant
)

/**
 * Distance metric types for vector search.
 */
enum class DistanceType {
    @JsonProperty("cosine") COSINE,
    @JsonProperty("euclidean") EUCLIDEAN,
    @JsonProperty("dot_product") DOT_PRODUCT
}

/**
 * A vector record for storage.
 */
data class VectorRecord(
    val id: String,
    val vector: List<Double>,
    val metadata: Map<String, Any?>? = null
)

/**
 * Result from vector search.
 */
data class VectorSearchResult(
    val id: String,
    val score: Double,
    val vector: List<Double>? = null,
    val metadata: Map<String, Any?>? = null
)

/**
 * Response from vector search.
 */
data class VectorSearchResponse(
    val results: List<VectorSearchResult>,
    val took: Long
)

/**
 * Response from upsert operation.
 */
data class UpsertResponse(
    @JsonProperty("upserted_count") val upsertedCount: Int
)

/**
 * Health status of the server.
 */
data class HealthStatus(
    val status: String,
    val components: Map<String, ComponentHealth>,
    val version: String? = null,
    val uptime: Long? = null
)

/**
 * Health of a component.
 */
data class ComponentHealth(
    val status: String,
    val message: String? = null,
    val latency: Long? = null
)

/**
 * Aggregation function types.
 */
enum class AggFunction {
    @JsonProperty("count") COUNT,
    @JsonProperty("sum") SUM,
    @JsonProperty("avg") AVG,
    @JsonProperty("min") MIN,
    @JsonProperty("max") MAX,
    @JsonProperty("last") LAST
}

/**
 * Response from aggregation query.
 */
data class AggregationResponse(
    @JsonProperty("entity_id") val entityId: String,
    val feature: String,
    val function: AggFunction,
    val value: Double,
    @JsonProperty("window_start") val windowStart: Instant,
    @JsonProperty("window_end") val windowEnd: Instant
)

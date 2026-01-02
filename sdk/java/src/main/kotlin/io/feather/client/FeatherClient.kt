package io.feather.client

import com.fasterxml.jackson.databind.DeserializationFeature
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule
import com.fasterxml.jackson.module.kotlin.KotlinModule
import com.fasterxml.jackson.module.kotlin.readValue
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.time.Duration
import java.time.Instant
import java.util.concurrent.TimeUnit

/**
 * Configuration options for the Feather client.
 */
data class ClientConfig(
    /** Base URL of the Feather server */
    val baseUrl: String,
    /** Request timeout */
    val timeout: Duration = Duration.ofSeconds(30),
    /** API key for authentication */
    val apiKey: String? = null,
    /** Additional headers */
    val headers: Map<String, String> = emptyMap(),
    /** Maximum retries for failed requests */
    val maxRetries: Int = 3,
    /** Initial retry delay */
    val initialRetryDelay: Duration = Duration.ofMillis(100),
    /** Maximum retry delay */
    val maxRetryDelay: Duration = Duration.ofSeconds(5)
)

/**
 * Feather Feature Store client for Java/Kotlin.
 *
 * Example usage:
 * ```kotlin
 * val client = FeatherClient(ClientConfig(baseUrl = "http://localhost:8080"))
 *
 * // Get features
 * val features = client.getFeatures("user:123", listOf("age", "country"))
 *
 * // Store features
 * client.putFeatures("user:123", mapOf("age" to 25, "country" to "US"))
 *
 * // Vector search
 * val results = client.vectors.search("embeddings", listOf(0.1, 0.2, ...), 10)
 * ```
 */
class FeatherClient(private val config: ClientConfig) {

    private val httpClient: OkHttpClient
    private val objectMapper: ObjectMapper
    private val baseUrl: String = config.baseUrl.trimEnd('/')

    /** Vector operations client */
    val vectors: VectorClient

    init {
        httpClient = OkHttpClient.Builder()
            .connectTimeout(config.timeout.toMillis(), TimeUnit.MILLISECONDS)
            .readTimeout(config.timeout.toMillis(), TimeUnit.MILLISECONDS)
            .writeTimeout(config.timeout.toMillis(), TimeUnit.MILLISECONDS)
            .addInterceptor { chain ->
                val builder = chain.request().newBuilder()
                    .addHeader("Content-Type", "application/json")
                    .addHeader("Accept", "application/json")

                config.apiKey?.let {
                    builder.addHeader("Authorization", "Bearer $it")
                }

                config.headers.forEach { (key, value) ->
                    builder.addHeader(key, value)
                }

                chain.proceed(builder.build())
            }
            .build()

        objectMapper = ObjectMapper()
            .registerModule(KotlinModule.Builder().build())
            .registerModule(JavaTimeModule())
            .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)

        vectors = VectorClient(this)
    }

    /**
     * Get features for an entity.
     */
    fun getFeatures(
        entityId: String,
        featureNames: List<String>? = null
    ): GetFeaturesResponse {
        val urlBuilder = HttpUrl.Builder()
            .scheme(if (baseUrl.startsWith("https")) "https" else "http")
            .host(baseUrl.removePrefix("https://").removePrefix("http://").substringBefore(":").substringBefore("/"))
            .port(baseUrl.substringAfter(":").substringBefore("/").toIntOrNull() ?: 8080)
            .addPathSegments("v1/features")
            .addQueryParameter("entity", entityId)

        featureNames?.forEach { urlBuilder.addQueryParameter("feature", it) }

        return get(urlBuilder.build().toString())
    }

    /**
     * Store features for an entity.
     */
    fun putFeatures(
        entityId: String,
        features: Map<String, FeatureValue>,
        timestamp: Instant? = null
    ) {
        val body = mapOf(
            "entity_id" to entityId,
            "features" to features,
            "timestamp" to timestamp?.toString()
        )
        post<Unit>("$baseUrl/v1/features", body)
    }

    /**
     * Get features for multiple entities.
     */
    fun batchGet(
        entityIds: List<String>,
        featureNames: List<String>? = null
    ): BatchGetResponse {
        val body = mapOf(
            "entities" to entityIds,
            "features" to featureNames
        )
        return post("$baseUrl/v1/features/batch", body)
    }

    /**
     * Get features at a specific point in time.
     */
    fun getFeaturesAsOf(
        entityId: String,
        asOf: Instant,
        featureNames: List<String>? = null
    ): GetFeaturesResponse {
        val urlBuilder = HttpUrl.Builder()
            .scheme(if (baseUrl.startsWith("https")) "https" else "http")
            .host(baseUrl.removePrefix("https://").removePrefix("http://").substringBefore(":").substringBefore("/"))
            .port(baseUrl.substringAfter(":").substringBefore("/").toIntOrNull() ?: 8080)
            .addPathSegments("v1/features/history")
            .addQueryParameter("entity", entityId)
            .addQueryParameter("as_of", asOf.toString())

        featureNames?.forEach { urlBuilder.addQueryParameter("feature", it) }

        return get(urlBuilder.build().toString())
    }

    /**
     * Get aggregated feature value.
     */
    fun getAggregation(
        entityId: String,
        feature: String,
        function: AggFunction,
        windowSeconds: Long
    ): AggregationResponse {
        val url = HttpUrl.Builder()
            .scheme(if (baseUrl.startsWith("https")) "https" else "http")
            .host(baseUrl.removePrefix("https://").removePrefix("http://").substringBefore(":").substringBefore("/"))
            .port(baseUrl.substringAfter(":").substringBefore("/").toIntOrNull() ?: 8080)
            .addPathSegments("v1/aggregation")
            .addQueryParameter("entity", entityId)
            .addQueryParameter("feature", feature)
            .addQueryParameter("function", function.name.lowercase())
            .addQueryParameter("window", windowSeconds.toString())
            .build()

        return get(url.toString())
    }

    /**
     * List all feature groups.
     */
    fun listFeatureGroups(): List<FeatureGroup> {
        val response: Map<String, List<FeatureGroup>> = get("$baseUrl/v1/schema/groups")
        return response["groups"] ?: emptyList()
    }

    /**
     * Get a feature group by name.
     */
    fun getFeatureGroup(name: String): FeatureGroup? {
        return try {
            get("$baseUrl/v1/schema/groups/$name")
        } catch (e: NotFoundException) {
            null
        }
    }

    /**
     * Check server health.
     */
    fun health(): HealthStatus {
        return get("$baseUrl/health")
    }

    /**
     * Check if server is ready.
     */
    fun ready(): Boolean {
        return try {
            get<Map<String, Any>>("$baseUrl/ready")
            true
        } catch (e: Exception) {
            false
        }
    }

    // HTTP helpers

    internal inline fun <reified T> get(url: String): T {
        return request("GET", url, null)
    }

    internal inline fun <reified T> post(url: String, body: Any?): T {
        return request("POST", url, body)
    }

    internal inline fun <reified T> delete(url: String): T {
        return request("DELETE", url, null)
    }

    internal inline fun <reified T> request(method: String, url: String, body: Any?): T {
        var lastException: Exception? = null
        var delay = config.initialRetryDelay.toMillis()

        repeat(config.maxRetries + 1) { attempt ->
            try {
                val requestBody = body?.let {
                    objectMapper.writeValueAsString(it)
                        .toRequestBody("application/json".toMediaType())
                }

                val request = Request.Builder()
                    .url(url)
                    .method(method, requestBody)
                    .build()

                httpClient.newCall(request).execute().use { response ->
                    val responseBody = response.body?.string() ?: ""

                    if (!response.isSuccessful) {
                        val errorMessage = try {
                            val errorMap = objectMapper.readValue<Map<String, String>>(responseBody)
                            errorMap["error"] ?: errorMap["message"] ?: responseBody
                        } catch (e: Exception) {
                            responseBody
                        }

                        when (response.code) {
                            404 -> throw NotFoundException(errorMessage)
                            400 -> throw ValidationException(errorMessage)
                            401 -> throw AuthenticationException(errorMessage)
                            429 -> throw RateLimitException(errorMessage)
                            in 500..599 -> {
                                if (attempt < config.maxRetries) {
                                    lastException = FeatherException(errorMessage, response.code)
                                    Thread.sleep(delay)
                                    delay = minOf(delay * 2, config.maxRetryDelay.toMillis())
                                    return@use
                                }
                                throw FeatherException(errorMessage, response.code)
                            }
                            else -> throw FeatherException(errorMessage, response.code)
                        }
                    }

                    return if (responseBody.isBlank() || T::class == Unit::class) {
                        @Suppress("UNCHECKED_CAST")
                        Unit as T
                    } else {
                        objectMapper.readValue(responseBody)
                    }
                }
            } catch (e: IOException) {
                lastException = ConnectionException("Connection failed: ${e.message}", e)
                if (attempt < config.maxRetries) {
                    Thread.sleep(delay)
                    delay = minOf(delay * 2, config.maxRetryDelay.toMillis())
                }
            } catch (e: FeatherException) {
                throw e
            }
        }

        throw lastException ?: ConnectionException("Request failed after retries")
    }

    /**
     * Close the client and release resources.
     */
    fun close() {
        httpClient.dispatcher.executorService.shutdown()
        httpClient.connectionPool.evictAll()
    }
}

/**
 * Vector operations client.
 */
class VectorClient internal constructor(private val client: FeatherClient) {

    private val baseUrl: String get() = "${client.javaClass.getDeclaredField("baseUrl").apply { isAccessible = true }.get(client)}"

    /**
     * List all vector indexes.
     */
    fun listIndexes(): List<String> {
        val response: Map<String, List<String>> = client.get("$baseUrl/v1/vectors")
        return response["indexes"] ?: emptyList()
    }

    /**
     * Create a new vector index.
     */
    fun createIndex(
        name: String,
        dimension: Int,
        distanceType: DistanceType = DistanceType.COSINE
    ): VectorIndex {
        return client.post("$baseUrl/v1/vectors", mapOf(
            "name" to name,
            "dimension" to dimension,
            "distance_type" to distanceType.name.lowercase()
        ))
    }

    /**
     * Get information about a vector index.
     */
    fun getIndex(name: String): VectorIndex {
        return client.get("$baseUrl/v1/vectors/$name")
    }

    /**
     * Delete a vector index.
     */
    fun deleteIndex(name: String) {
        client.delete<Unit>("$baseUrl/v1/vectors/$name")
    }

    /**
     * Upsert vectors into an index.
     */
    fun upsert(index: String, vectors: List<VectorRecord>): Int {
        val response: UpsertResponse = client.post(
            "$baseUrl/v1/vectors/$index/upsert",
            mapOf("vectors" to vectors)
        )
        return response.upsertedCount
    }

    /**
     * Search for similar vectors.
     */
    fun search(
        index: String,
        vector: List<Double>,
        topK: Int = 10,
        filter: Map<String, Any?>? = null,
        includeMetadata: Boolean = true,
        includeVector: Boolean = false
    ): List<VectorSearchResult> {
        val response: VectorSearchResponse = client.post(
            "$baseUrl/v1/vectors/$index/search",
            mapOf(
                "vector" to vector,
                "top_k" to topK,
                "filter" to filter,
                "include_metadata" to includeMetadata,
                "include_vector" to includeVector
            )
        )
        return response.results
    }

    /**
     * Get a vector by ID.
     */
    fun get(index: String, id: String): VectorRecord? {
        return try {
            client.get("$baseUrl/v1/vectors/$index/$id")
        } catch (e: NotFoundException) {
            null
        }
    }

    /**
     * Delete a vector by ID.
     */
    fun delete(index: String, id: String) {
        client.delete<Unit>("$baseUrl/v1/vectors/$index/$id")
    }
}

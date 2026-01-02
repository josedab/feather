package io.feather.client

import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.jupiter.api.*
import org.junit.jupiter.api.Assertions.*
import java.time.Duration
import java.time.Instant

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class FeatherClientTest {

    private lateinit var mockServer: MockWebServer
    private lateinit var client: FeatherClient

    @BeforeAll
    fun setup() {
        mockServer = MockWebServer()
        mockServer.start()

        client = FeatherClient(ClientConfig(
            baseUrl = mockServer.url("/").toString(),
            timeout = Duration.ofSeconds(5),
            maxRetries = 0
        ))
    }

    @AfterAll
    fun teardown() {
        client.close()
        mockServer.shutdown()
    }

    @Test
    fun `getFeatures should return features for entity`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"entity_id": "user:123", "features": {"age": 25, "country": "US"}}""")
            .setHeader("Content-Type", "application/json"))

        val response = client.getFeatures("user:123")

        assertEquals("user:123", response.entityId)
        assertEquals(25, (response.features["age"] as Number).toInt())
        assertEquals("US", response.features["country"])
    }

    @Test
    fun `getFeatures with feature names should include in request`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"entity_id": "user:123", "features": {"age": 25}}""")
            .setHeader("Content-Type", "application/json"))

        client.getFeatures("user:123", listOf("age", "country"))

        val request = mockServer.takeRequest()
        assertTrue(request.path!!.contains("feature=age"))
        assertTrue(request.path!!.contains("feature=country"))
    }

    @Test
    fun `putFeatures should send features to server`() {
        mockServer.enqueue(MockResponse().setResponseCode(200))

        client.putFeatures("user:123", mapOf("age" to 25, "country" to "US"))

        val request = mockServer.takeRequest()
        assertEquals("POST", request.method)
        assertTrue(request.path!!.contains("/v1/features"))
        assertTrue(request.body.readUtf8().contains("\"entity_id\":\"user:123\""))
    }

    @Test
    fun `batchGet should return features for multiple entities`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"results": [{"entity_id": "user:1", "features": {"age": 20}}, {"entity_id": "user:2", "features": {"age": 30}}]}""")
            .setHeader("Content-Type", "application/json"))

        val response = client.batchGet(listOf("user:1", "user:2"))

        assertEquals(2, response.results.size)
    }

    @Test
    fun `health should return server health status`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"status": "healthy", "components": {"storage": {"status": "healthy"}}}""")
            .setHeader("Content-Type", "application/json"))

        val health = client.health()

        assertEquals("healthy", health.status)
        assertTrue(health.components.containsKey("storage"))
    }

    @Test
    fun `ready should return true when server is ready`() {
        mockServer.enqueue(MockResponse().setResponseCode(200).setBody("{}"))

        assertTrue(client.ready())
    }

    @Test
    fun `ready should return false when server is not ready`() {
        mockServer.enqueue(MockResponse().setResponseCode(503))

        assertFalse(client.ready())
    }

    @Test
    fun `should throw NotFoundException for 404`() {
        mockServer.enqueue(MockResponse()
            .setResponseCode(404)
            .setBody("""{"error": "Entity not found"}"""))

        assertThrows<NotFoundException> {
            client.getFeatures("unknown")
        }
    }

    @Test
    fun `should throw ValidationException for 400`() {
        mockServer.enqueue(MockResponse()
            .setResponseCode(400)
            .setBody("""{"error": "Invalid request"}"""))

        assertThrows<ValidationException> {
            client.putFeatures("", emptyMap())
        }
    }
}

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class VectorClientTest {

    private lateinit var mockServer: MockWebServer
    private lateinit var client: FeatherClient

    @BeforeAll
    fun setup() {
        mockServer = MockWebServer()
        mockServer.start()

        client = FeatherClient(ClientConfig(
            baseUrl = mockServer.url("/").toString(),
            maxRetries = 0
        ))
    }

    @AfterAll
    fun teardown() {
        client.close()
        mockServer.shutdown()
    }

    @Test
    fun `listIndexes should return index names`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"indexes": ["index1", "index2"]}""")
            .setHeader("Content-Type", "application/json"))

        val indexes = client.vectors.listIndexes()

        assertEquals(listOf("index1", "index2"), indexes)
    }

    @Test
    fun `createIndex should create a new index`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"name": "test", "dimension": 128, "distance_type": "cosine", "vector_count": 0, "created_at": "2024-01-01T00:00:00Z"}""")
            .setHeader("Content-Type", "application/json"))

        val index = client.vectors.createIndex("test", 128)

        assertEquals("test", index.name)
        assertEquals(128, index.dimension)
        assertEquals(DistanceType.COSINE, index.distanceType)
    }

    @Test
    fun `search should return similar vectors`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"results": [{"id": "v1", "score": 0.95, "metadata": {"label": "test"}}], "took": 5}""")
            .setHeader("Content-Type", "application/json"))

        val results = client.vectors.search("index", listOf(0.1, 0.2, 0.3), 10)

        assertEquals(1, results.size)
        assertEquals("v1", results[0].id)
        assertEquals(0.95, results[0].score)
    }

    @Test
    fun `upsert should return upserted count`() {
        mockServer.enqueue(MockResponse()
            .setBody("""{"upserted_count": 2}""")
            .setHeader("Content-Type", "application/json"))

        val count = client.vectors.upsert("index", listOf(
            VectorRecord("v1", listOf(0.1, 0.2)),
            VectorRecord("v2", listOf(0.3, 0.4))
        ))

        assertEquals(2, count)
    }

    @Test
    fun `get should return null for non-existent vector`() {
        mockServer.enqueue(MockResponse()
            .setResponseCode(404)
            .setBody("""{"error": "Vector not found"}"""))

        val result = client.vectors.get("index", "nonexistent")

        assertNull(result)
    }
}

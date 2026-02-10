/**
 * Feather Kotlin Quickstart - Get started in 30 seconds!
 */
package dev.feather.quickstart

import dev.feather.client.FeatherClient
import dev.feather.client.PutFeaturesRequest
import java.net.HttpURLConnection
import java.net.URI
import kotlin.system.exitProcess

suspend fun main() {
    val featherUrl = "http://localhost:8080"

    // Preflight: ensure the Feather server is reachable
    try {
        val conn = URI("$featherUrl/health").toURL().openConnection() as HttpURLConnection
        conn.connectTimeout = 3000
        conn.readTimeout = 3000
        conn.responseCode
        conn.disconnect()
    } catch (e: Exception) {
        System.err.println("❌ Cannot connect to Feather at $featherUrl")
        System.err.println("   Start the server first:  make run-dev")
        exitProcess(1)
    }

    // 1. Connect to Feather
    val client = FeatherClient(featherUrl)

    // 2. Store features for an entity
    client.putFeatures(
        PutFeaturesRequest(
            entityId = "user:123",
            features = mapOf(
                "score" to 0.95,
                "purchases" to 42,
                "premium" to true
            )
        )
    )
    println("Stored features for user:123")

    // 3. Retrieve features
    val response = client.getFeatures("user:123", listOf("score", "purchases"))
    println("Retrieved features for ${response.entityId}:")
    response.features.forEach { (name, fv) ->
        println("  $name: ${fv.value} (updated: ${fv.timestamp})")
    }

    // 4. Batch retrieval (multiple entities)
    val results = client.getFeaturesBatch(
        entityIds = listOf("user:123", "user:456"),
        features = listOf("score")
    )
    println("\nBatch retrieved ${results.size} entities")

    println("\nQuickstart complete!")

    client.close()
}

/**
 * Feather Kotlin Quickstart - Get started in 30 seconds!
 */
package dev.feather.quickstart

import dev.feather.client.FeatherClient
import dev.feather.client.PutFeaturesRequest

suspend fun main() {
    // 1. Connect to Feather
    val client = FeatherClient("http://localhost:8080")

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

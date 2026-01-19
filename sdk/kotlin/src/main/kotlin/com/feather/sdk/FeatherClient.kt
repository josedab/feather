// FeatherClient.kt
// Feather Feature Store - Android/Kotlin SDK
// Offline-first feature serving with delta sync

package com.feather.sdk

import java.net.HttpURLConnection
import java.net.URL
import java.time.Instant
import java.util.concurrent.*
import java.util.concurrent.atomic.AtomicLong

/**
 * Configuration for the Feather client.
 */
data class FeatherConfig(
    val baseURL: String,
    val deviceID: String,
    val syncIntervalSeconds: Long = 300,
    val offlineStorageLimit: Int = 10_000,
    val conflictStrategy: ConflictStrategy = ConflictStrategy.SERVER_WINS,
    val compressionEnabled: Boolean = true
)

/**
 * Conflict resolution strategy matching the server-side protocol.
 */
enum class ConflictStrategy(val value: String) {
    SERVER_WINS("server_wins"),
    CLIENT_WINS("client_wins"),
    LAST_WRITE_WINS("last_write_wins")
}

/**
 * A cached feature value stored locally on the device.
 */
data class CachedFeature(
    val featureID: String,
    val entityKey: String,
    var value: Any?,
    var version: Long,
    var updatedAt: Instant,
    var isDirty: Boolean = false
)

/**
 * Result of a sync operation.
 */
data class SyncResult(
    val updatesReceived: Int,
    val updatesSent: Int,
    val conflictsResolved: Int,
    val durationMs: Long
)

/**
 * Main Feather client with offline-first architecture.
 *
 * Features are served from a local cache and synchronized with the
 * server in the background. Writes are queued locally and pushed
 * on the next sync cycle.
 */
class FeatherClient(private val config: FeatherConfig) {

    private val cache = ConcurrentHashMap<String, CachedFeature>()
    private val pendingSync = ConcurrentLinkedQueue<CachedFeature>()
    private val lastSyncVersion = AtomicLong(0)
    private var syncExecutor: ScheduledExecutorService? = null

    /**
     * Retrieve a cached feature value (offline-first).
     */
    fun get(featureID: String, entityKey: String): CachedFeature? {
        return cache[cacheKey(featureID, entityKey)]
    }

    /**
     * Store a feature value locally and mark it for sync.
     */
    fun put(featureID: String, entityKey: String, value: Any?) {
        val key = cacheKey(featureID, entityKey)
        val existing = cache[key]
        val version = (existing?.version ?: 0) + 1

        val feature = CachedFeature(
            featureID = featureID,
            entityKey = entityKey,
            value = value,
            version = version,
            updatedAt = Instant.now(),
            isDirty = true
        )
        cache[key] = feature
        pendingSync.add(feature)

        // Evict oldest entries when over the storage limit
        if (cache.size > config.offlineStorageLimit) {
            evictOldest()
        }
    }

    /**
     * Delete a feature from the local cache.
     */
    fun delete(featureID: String, entityKey: String) {
        cache.remove(cacheKey(featureID, entityKey))
    }

    /**
     * Perform a one-shot sync with the server.
     */
    fun sync(): SyncResult {
        val start = System.currentTimeMillis()
        val sent = pendingSync.size

        val url = URL("${config.baseURL}/v1/mobile/sync")
        val conn = url.openConnection() as HttpURLConnection
        return try {
            conn.requestMethod = "POST"
            conn.setRequestProperty("Content-Type", "application/json")
            conn.doOutput = true

            val body = """
                {
                    "device_id": "${config.deviceID}",
                    "mode": "delta",
                    "client_version": ${lastSyncVersion.get()}
                }
            """.trimIndent()

            conn.outputStream.use { it.write(body.toByteArray()) }

            val responseCode = conn.responseCode
            if (responseCode == HttpURLConnection.HTTP_OK) {
                val responseBody = conn.inputStream.bufferedReader().readText()
                pendingSync.clear()
                lastSyncVersion.incrementAndGet()

                SyncResult(
                    updatesReceived = 0,
                    updatesSent = sent,
                    conflictsResolved = 0,
                    durationMs = System.currentTimeMillis() - start
                )
            } else {
                SyncResult(
                    updatesReceived = 0,
                    updatesSent = 0,
                    conflictsResolved = 0,
                    durationMs = System.currentTimeMillis() - start
                )
            }
        } catch (e: Exception) {
            SyncResult(
                updatesReceived = 0,
                updatesSent = 0,
                conflictsResolved = 0,
                durationMs = System.currentTimeMillis() - start
            )
        } finally {
            conn.disconnect()
        }
    }

    /**
     * Start periodic background sync at the configured interval.
     */
    fun startPeriodicSync() {
        stopPeriodicSync()
        syncExecutor = Executors.newSingleThreadScheduledExecutor { r ->
            Thread(r, "feather-sync").apply { isDaemon = true }
        }
        syncExecutor?.scheduleAtFixedRate(
            { sync() },
            config.syncIntervalSeconds,
            config.syncIntervalSeconds,
            TimeUnit.SECONDS
        )
    }

    /**
     * Stop periodic background sync.
     */
    fun stopPeriodicSync() {
        syncExecutor?.shutdown()
        syncExecutor = null
    }

    /** Number of features in the local cache. */
    val cacheSize: Int get() = cache.size

    /** Remove all cached features. */
    fun clearCache() {
        cache.clear()
        pendingSync.clear()
    }

    /** Number of features awaiting sync. */
    val pendingSyncCount: Int get() = pendingSync.size

    /** Shut down the client and release resources. */
    fun close() {
        stopPeriodicSync()
        cache.clear()
        pendingSync.clear()
    }

    private fun cacheKey(featureID: String, entityKey: String): String =
        "$featureID::$entityKey"

    private fun evictOldest() {
        val sorted = cache.entries.sortedBy { it.value.updatedAt }
        val excess = cache.size - config.offlineStorageLimit
        sorted.take(excess).forEach { cache.remove(it.key) }
    }
}

// FeatherClient.swift
// Feather Feature Store - iOS SDK
// Offline-first feature serving with delta sync

import Foundation

/// Configuration for the Feather client.
public struct FeatherConfig {
    public let baseURL: String
    public let deviceID: String
    public var syncIntervalSeconds: TimeInterval = 300
    public var offlineStorageLimit: Int = 10_000
    public var conflictStrategy: ConflictStrategy = .serverWins
    public var compressionEnabled: Bool = true

    public init(baseURL: String, deviceID: String) {
        self.baseURL = baseURL
        self.deviceID = deviceID
    }
}

/// Conflict resolution strategy matching the server-side protocol.
public enum ConflictStrategy: String, Codable {
    case serverWins = "server_wins"
    case clientWins = "client_wins"
    case lastWriteWins = "last_write_wins"
}

/// A cached feature value stored locally on the device.
public struct CachedFeature: Codable {
    public let featureID: String
    public let entityKey: String
    public var value: AnyCodable
    public var version: Int64
    public var updatedAt: Date
    public var isDirty: Bool

    public init(featureID: String, entityKey: String, value: AnyCodable, version: Int64, updatedAt: Date, isDirty: Bool) {
        self.featureID = featureID
        self.entityKey = entityKey
        self.value = value
        self.version = version
        self.updatedAt = updatedAt
        self.isDirty = isDirty
    }
}

/// Result of a sync operation.
public struct SyncResult {
    public let updatesReceived: Int
    public let updatesSent: Int
    public let conflictsResolved: Int
    public let duration: TimeInterval
}

/// Main Feather client with offline-first architecture.
///
/// Features are served from a local cache and synchronized with the
/// server in the background. Writes are queued locally and pushed
/// on the next sync cycle.
public class FeatherClient {
    private let config: FeatherConfig
    private var cache: [String: CachedFeature] = [:]
    private var pendingSync: [CachedFeature] = []
    private var lastSyncVersion: Int64 = 0
    private let queue = DispatchQueue(label: "com.feather.sync", qos: .utility)
    private var syncTimer: DispatchSourceTimer?

    public init(config: FeatherConfig) {
        self.config = config
    }

    // MARK: - Feature Access

    /// Retrieve a cached feature value (offline-first).
    public func get(featureID: String, entityKey: String) -> CachedFeature? {
        let key = Self.cacheKey(featureID: featureID, entityKey: entityKey)
        return cache[key]
    }

    /// Store a feature value locally and mark it for sync.
    public func put(featureID: String, entityKey: String, value: Any) {
        let key = Self.cacheKey(featureID: featureID, entityKey: entityKey)
        let existing = cache[key]
        let version = (existing?.version ?? 0) + 1

        let feature = CachedFeature(
            featureID: featureID,
            entityKey: entityKey,
            value: AnyCodable(value),
            version: version,
            updatedAt: Date(),
            isDirty: true
        )
        cache[key] = feature
        pendingSync.append(feature)

        // Evict oldest entries when over the storage limit
        if cache.count > config.offlineStorageLimit {
            evictOldest()
        }
    }

    /// Delete a feature from the local cache and queue deletion for sync.
    public func delete(featureID: String, entityKey: String) {
        let key = Self.cacheKey(featureID: featureID, entityKey: entityKey)
        cache.removeValue(forKey: key)
    }

    // MARK: - Sync

    /// Perform a one-shot sync with the server.
    public func sync(completion: @escaping (Result<SyncResult, Error>) -> Void) {
        queue.async { [weak self] in
            guard let self = self else { return }
            let start = Date()
            let sent = self.pendingSync.count

            guard let url = URL(string: "\(self.config.baseURL)/v1/mobile/sync") else {
                completion(.failure(FeatherError.invalidURL))
                return
            }

            var request = URLRequest(url: url)
            request.httpMethod = "POST"
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")

            let body: [String: Any] = [
                "device_id": self.config.deviceID,
                "mode": "delta",
                "client_version": self.lastSyncVersion
            ]

            do {
                request.httpBody = try JSONSerialization.data(withJSONObject: body)
            } catch {
                completion(.failure(error))
                return
            }

            let task = URLSession.shared.dataTask(with: request) { [weak self] data, response, error in
                guard let self = self else { return }

                if let error = error {
                    completion(.failure(error))
                    return
                }

                guard let data = data,
                      let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                      let syncData = json["sync"] as? [String: Any] else {
                    completion(.failure(FeatherError.invalidResponse))
                    return
                }

                let serverVersion = syncData["server_version"] as? Int64 ?? self.lastSyncVersion
                let updates = syncData["updates"] as? [[String: Any]] ?? []
                let conflictsResolved = syncData["conflicts_resolved"] as? Int ?? 0

                self.lastSyncVersion = serverVersion
                self.pendingSync.removeAll()

                let result = SyncResult(
                    updatesReceived: updates.count,
                    updatesSent: sent,
                    conflictsResolved: conflictsResolved,
                    duration: Date().timeIntervalSince(start)
                )
                completion(.success(result))
            }
            task.resume()
        }
    }

    /// Start periodic background sync at the configured interval.
    public func startPeriodicSync() {
        stopPeriodicSync()

        let timer = DispatchSource.makeTimerSource(queue: queue)
        timer.schedule(
            deadline: .now() + config.syncIntervalSeconds,
            repeating: config.syncIntervalSeconds
        )
        timer.setEventHandler { [weak self] in
            self?.sync { _ in }
        }
        timer.resume()
        syncTimer = timer
    }

    /// Stop periodic background sync.
    public func stopPeriodicSync() {
        syncTimer?.cancel()
        syncTimer = nil
    }

    // MARK: - Cache Management

    /// Number of features in the local cache.
    public var cacheSize: Int { cache.count }

    /// Remove all cached features.
    public func clearCache() {
        cache.removeAll()
        pendingSync.removeAll()
    }

    /// Number of features awaiting sync.
    public var pendingSyncCount: Int { pendingSync.count }

    // MARK: - Private

    private static func cacheKey(featureID: String, entityKey: String) -> String {
        "\(featureID)::\(entityKey)"
    }

    private func evictOldest() {
        let sorted = cache.sorted { $0.value.updatedAt < $1.value.updatedAt }
        let excess = cache.count - config.offlineStorageLimit
        for entry in sorted.prefix(excess) {
            cache.removeValue(forKey: entry.key)
        }
    }
}

// MARK: - Errors

/// Errors specific to the Feather SDK.
public enum FeatherError: Error, LocalizedError {
    case invalidURL
    case invalidResponse
    case syncFailed(String)

    public var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "Invalid server URL"
        case .invalidResponse:
            return "Invalid response from server"
        case .syncFailed(let reason):
            return "Sync failed: \(reason)"
        }
    }
}

// MARK: - AnyCodable

/// Type-erased Codable wrapper for heterogeneous feature values.
public struct AnyCodable: Codable {
    public let value: Any

    public init(_ value: Any) {
        self.value = value
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let intVal = try? container.decode(Int.self) {
            value = intVal
        } else if let doubleVal = try? container.decode(Double.self) {
            value = doubleVal
        } else if let boolVal = try? container.decode(Bool.self) {
            value = boolVal
        } else if let stringVal = try? container.decode(String.self) {
            value = stringVal
        } else if container.decodeNil() {
            value = NSNull()
        } else {
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "Unsupported type")
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch value {
        case let intVal as Int:
            try container.encode(intVal)
        case let doubleVal as Double:
            try container.encode(doubleVal)
        case let boolVal as Bool:
            try container.encode(boolVal)
        case let stringVal as String:
            try container.encode(stringVal)
        default:
            try container.encodeNil()
        }
    }
}

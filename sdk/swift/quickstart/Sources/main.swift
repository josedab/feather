// Feather Swift Quickstart — Get started in 30 seconds!

import Foundation
import FeatherSDK

// 1. Configure and create a client
var config = FeatherConfig(baseURL: "http://localhost:8080", deviceID: "quickstart-device")
config.syncIntervalSeconds = 60
config.offlineStorageLimit = 5_000
config.conflictStrategy = .serverWins

let client = FeatherClient(config: config)

// 2. Store features locally (queued for sync)
client.put(featureID: "score", entityKey: "user:123", value: 0.95)
client.put(featureID: "purchases", entityKey: "user:123", value: 42)
client.put(featureID: "premium", entityKey: "user:123", value: true)
print("✅ Stored features for user:123")

// 3. Retrieve features from local cache (instant, works offline)
if let feature = client.get(featureID: "score", entityKey: "user:123") {
    print("   score: \(feature.value.value) (updated: \(feature.updatedAt))")
}
if let feature = client.get(featureID: "purchases", entityKey: "user:123") {
    print("   purchases: \(feature.value.value) (updated: \(feature.updatedAt))")
}

// 4. Check cache stats
print("\nCache size: \(client.cacheSize)")
print("Pending sync: \(client.pendingSyncCount)")

// 5. Trigger a sync with the server
print("\nSyncing with server...")
let semaphore = DispatchSemaphore(value: 0)
client.sync { result in
    switch result {
    case .success(let syncResult):
        print("✅ Sync complete — received: \(syncResult.updatesReceived), sent: \(syncResult.updatesSent)")
    case .failure(let error):
        print("⚠️  Sync failed (expected if server is not running): \(error.localizedDescription)")
    }
    semaphore.signal()
}
semaphore.wait()

print("\nQuickstart complete!")

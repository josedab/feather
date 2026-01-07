"""Feather Python Quickstart - Get started in 30 seconds!"""

from feather_client import FeatherClient

# 1. Connect to Feather
client = FeatherClient("http://localhost:8080")

# 2. Store features for an entity
client.put_features(
    entity_id="user:123",
    features={
        "score": 0.95,
        "purchases": 42,
        "premium": True,
    }
)
print("Stored features for user:123")

# 3. Retrieve features
response = client.get_features("user:123", features=["score", "purchases"])
print(f"Retrieved features for {response.entity_id}:")
for name, fv in response.features.items():
    print(f"  {name}: {fv.value} (updated: {fv.timestamp})")

# 4. Batch retrieval (multiple entities)
results = client.get_features_batch(
    entity_ids=["user:123", "user:456"],
    features=["score"]
)
print(f"\nBatch retrieved {len(results)} entities")

print("\nQuickstart complete!")

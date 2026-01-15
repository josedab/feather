#!/usr/bin/env python3
"""
Feather End-to-End Example: ML Feature Pipeline

Demonstrates a realistic ML workflow using Feather's HTTP API:
  1. Store user behavior features (simulating real-time ingestion)
  2. Retrieve features for real-time inference
  3. Batch feature retrieval for model training
  4. Point-in-time queries for backtesting
  5. Vector similarity search for recommendations

Prerequisites:
  - Start Feather:  make run-dev
  - Python 3.9+ with urllib (stdlib only — no pip install needed)

Usage:
  python examples/ml-pipeline.py
"""

import json
import sys
import urllib.error
import urllib.request

BASE_URL = "http://localhost:8080"


def api(method, path, body=None):
    """Send a request to the Feather API and return parsed JSON."""
    url = f"{BASE_URL}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    if data:
        req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def section(title):
    print(f"\n{'─' * 60}")
    print(f"  {title}")
    print(f"{'─' * 60}")


def pp(obj):
    """Pretty-print a JSON object."""
    print(json.dumps(obj, indent=2))


# ── Preflight ─────────────────────────────────────────────────────

try:
    health = api("GET", "/health")
except (urllib.error.URLError, ConnectionRefusedError):
    print("❌ Cannot connect to Feather at", BASE_URL)
    print("   Start the server first:  make run-dev")
    sys.exit(1)

print("✅ Connected to Feather:", json.dumps(health))


# ── Step 1: Store User Behavior Features ──────────────────────────

section("Step 1: Ingesting user behavior features")

users = [
    {"entity_key": "user:alice", "features": {
        "click_count": 120, "purchase_total": 549.99,
        "last_activity": "2024-06-15T14:30:00Z"}},
    {"entity_key": "user:bob", "features": {
        "click_count": 45, "purchase_total": 89.50,
        "last_activity": "2024-06-14T09:15:00Z"}},
    {"entity_key": "user:carol", "features": {
        "click_count": 310, "purchase_total": 1299.00,
        "last_activity": "2024-06-15T18:45:00Z"}},
    {"entity_key": "user:dave", "features": {
        "click_count": 8, "purchase_total": 12.99,
        "last_activity": "2024-06-10T11:00:00Z"}},
    {"entity_key": "user:eve", "features": {
        "click_count": 200, "purchase_total": 780.00,
        "last_activity": "2024-06-15T20:00:00Z"}},
]

for user in users:
    resp = api("POST", "/v1/features", user)
    name = user["entity_key"]
    clicks = user["features"]["click_count"]
    total = user["features"]["purchase_total"]
    print(f"  ✓ {name}: clicks={clicks}, spend=${total:.2f}")


# ── Step 2: Real-Time Feature Retrieval (Inference) ───────────────

section("Step 2: Real-time feature retrieval (model inference)")

print("Fetching features for user:carol (high-value customer):")
resp = api("GET", "/v1/features?entity=user:carol&feature=click_count&feature=purchase_total")
pp(resp)

print("\nIn production, these features feed directly into your model:")
print("  model.predict(features=[click_count, purchase_total])")


# ── Step 3: Batch Feature Retrieval (Training) ───────────────────

section("Step 3: Batch retrieval for model training")

print("Fetching training data for 3 users in one call:")
resp = api("POST", "/v1/features/batch", {
    "entities": ["user:alice", "user:bob", "user:carol"],
    "features": ["click_count", "purchase_total"],
})
pp(resp)

print("\nBatch retrieval is optimized for training pipelines —")
print("fetch thousands of entities in a single round-trip.")


# ── Step 4: Point-in-Time Query (Backtesting) ────────────────────

section("Step 4: Point-in-time query (backtesting)")

print("What were alice's features as of 2024-06-15?")
resp = api("GET", "/v1/features/history?entity=user:alice&feature=click_count&as_of=2099-01-01T00:00:00Z")
pp(resp)

print("\nPoint-in-time queries prevent data leakage in training:")
print("  You only see features that existed at the query timestamp.")


# ── Step 5: Vector Similarity Search (Recommendations) ───────────

section("Step 5: Vector similarity search (recommendations)")

print("Creating a product embedding index...")
try:
    api("POST", "/v1/vectors", {
        "name": "product_recs",
        "dimension": 4,
        "distance_type": "cosine",
    })
    print("  ✓ Index 'product_recs' created")
except urllib.error.HTTPError:
    print("  ℹ Index 'product_recs' already exists")

print("\nUpserting product embeddings...")
api("POST", "/v1/vectors/product_recs/upsert", {
    "vectors": [
        {"id": "laptop_pro",    "vector": [0.9, 0.1, 0.8, 0.2]},
        {"id": "laptop_air",    "vector": [0.85, 0.15, 0.75, 0.25]},
        {"id": "phone_x",       "vector": [0.1, 0.9, 0.2, 0.8]},
        {"id": "tablet_mini",   "vector": [0.5, 0.5, 0.5, 0.5]},
    ],
})
print("  ✓ 4 product vectors upserted")

print("\nSearching for products similar to 'laptop_pro':")
results = api("POST", "/v1/vectors/product_recs/search", {
    "vector": [0.9, 0.1, 0.8, 0.2],
    "top_k": 3,
})
pp(results)

# Cleanup
try:
    api("DELETE", "/v1/vectors/product_recs")
except urllib.error.HTTPError:
    pass

print("\nVector search powers real-time recommendations,")
print("RAG retrieval, and semantic feature discovery.")


# ── Summary ───────────────────────────────────────────────────────

section("Done! 🎉")

print("""
You've seen the core Feather workflow:

  Ingest → Serve → Batch → Backtest → Recommend

Next steps:
  • Python SDK:     pip install -e sdk/python/
  • Go SDK:         cd sdk/go/feather/quickstart && go run main.go
  • Full API docs:  docs/api-reference.md
  • Run all tests:  make smoke-test
""")

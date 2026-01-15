#!/usr/bin/env python3
"""
Feather Example: Real-Time Fraud Detection

Demonstrates using Feather as the feature store for a fraud detection system:
  1. Ingest transaction features for multiple users
  2. Retrieve features in real-time for fraud scoring
  3. Batch retrieval for model retraining

Prerequisites:
  - Start Feather:  make run-dev
  - Python 3.9+ (stdlib only — no pip install needed)

Usage:
  python examples/fraud-detection.py
"""

import json
import sys
import urllib.error
import urllib.request

BASE_URL = "http://localhost:8080"


def api(method, path, body=None):
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
    print(json.dumps(obj, indent=2))


# ── Preflight ─────────────────────────────────────────────────────

try:
    health = api("GET", "/health")
except (urllib.error.URLError, ConnectionRefusedError):
    print("❌ Cannot connect to Feather at", BASE_URL)
    print("   Start the server first:  make run-dev")
    sys.exit(1)

print("✅ Connected to Feather")


# ── Step 1: Ingest transaction features ───────────────────────────

section("Step 1: Ingesting transaction features")

transactions = [
    {
        "entity_key": "user:alice",
        "features": {
            "click_count": 5,
            "purchase_total": 12500.00,
            "last_activity": "2024-06-15T23:45:00Z",
        },
    },
    {
        "entity_key": "user:bob",
        "features": {
            "click_count": 150,
            "purchase_total": 45.00,
            "last_activity": "2024-06-15T10:30:00Z",
        },
    },
    {
        "entity_key": "user:carol",
        "features": {
            "click_count": 3,
            "purchase_total": 8999.99,
            "last_activity": "2024-06-15T03:15:00Z",
        },
    },
]

for txn in transactions:
    api("POST", "/v1/features", txn)
    name = txn["entity_key"]
    total = txn["features"]["purchase_total"]
    clicks = txn["features"]["click_count"]
    print(f"  ✓ {name}: clicks={clicks}, total=${total:,.2f}")


# ── Step 2: Real-time fraud scoring ──────────────────────────────

section("Step 2: Real-time fraud scoring retrieval")

print("Retrieving features for user:alice (high-value transaction):")
resp = api(
    "GET",
    "/v1/features?entity=user:alice&feature=click_count&feature=purchase_total",
)
pp(resp)

print("\nIn a real system, these features feed your fraud model:")
print("  risk_score = model.predict(click_count, purchase_total)")
print("  → Low click count + high purchase = potential fraud signal")


# ── Step 3: Batch retrieval for retraining ────────────────────────

section("Step 3: Batch retrieval for model retraining")

print("Fetching training data for all users:")
resp = api(
    "POST",
    "/v1/features/batch",
    {
        "entities": ["user:alice", "user:bob", "user:carol"],
        "features": ["click_count", "purchase_total"],
    },
)
pp(resp)

print("\nBatch retrieval is optimized for training pipelines —")
print("fetch thousands of entities in a single round-trip.")


# ── Summary ───────────────────────────────────────────────────────

section("Done! 🎉")

print("""
Fraud detection pipeline complete:

  Ingest transactions → Score in real-time → Retrain in batch

This example used Feather's core APIs:
  • POST /v1/features          (ingest)
  • GET  /v1/features          (real-time retrieval)
  • POST /v1/features/batch    (batch retrieval)

Next steps:
  • Add drift detection:     docs/api-reference.md#drift
  • Point-in-time queries:   GET /v1/features/history
  • Python SDK:              pip install -e sdk/python/
""")

"""Feather Python SDK for defining feature transformations.

This SDK enables data scientists to write feature logic in Python
while Feather handles serving in Go with sub-millisecond latency.

Usage:
    from feather_sdk import FeatherClient, transform, on_demand

    @on_demand(inputs=["age"], outputs=["age_bucket"])
    def age_bucket(age: int) -> int:
        return age // 10 * 10

    client = FeatherClient("http://localhost:8080")
    client.register(age_bucket)
"""

__version__ = "0.1.0"

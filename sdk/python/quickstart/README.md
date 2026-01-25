# Feather Python Quickstart

Get started with Feather in 30 seconds.

## Prerequisites

- Python 3.9+
- Docker (for running Feather server)

## Step 1: Start Feather

```bash
docker run -d --name feather -p 8080:8080 ghcr.io/feather-store/feather:latest
```

## Step 2: Install the SDK

```bash
pip install feather-client
```

## Step 3: Run the Quickstart

```bash
python quickstart.py
```

## What This Does

1. Connects to Feather
2. Stores features for a user entity
3. Retrieves the features back
4. Demonstrates batch retrieval

## Async Support

```python
from feather_client import AsyncFeatherClient

async def main():
    client = AsyncFeatherClient("http://localhost:8080")
    features = await client.get_features("user:123")
```

## Next Steps

- Check out the [full documentation](https://feather-store.dev/docs)
- Explore [LangChain integration](https://feather-store.dev/docs/integrations/langchain)
- Learn about [vector similarity search](https://feather-store.dev/docs/vectors)

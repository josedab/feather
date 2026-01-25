# Feather TypeScript Quickstart

Get started with Feather in 30 seconds.

## Prerequisites

- Node.js 18+
- Docker (for running Feather server)

## Step 1: Start Feather

```bash
docker run -d --name feather -p 8080:8080 ghcr.io/feather-store/feather:latest
```

## Step 2: Install Dependencies

```bash
npm install
```

## Step 3: Run the Quickstart

```bash
npx ts-node quickstart.ts
```

Or with plain Node.js:

```bash
npx tsc && node quickstart.js
```

## What This Does

1. Connects to Feather
2. Stores features for a user entity
3. Retrieves the features back
4. Demonstrates batch retrieval

## Next Steps

- Check out the [full documentation](https://feather-store.dev/docs)
- Explore [vector similarity search](https://feather-store.dev/docs/vectors)
- Learn about [real-time streaming](https://feather-store.dev/docs/streaming)

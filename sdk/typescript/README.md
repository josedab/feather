# Feather TypeScript/JavaScript SDK

Official TypeScript/JavaScript client for [Feather Feature Store](https://github.com/feather-store/feather).

## Installation

```bash
npm install @feather/client
# or
yarn add @feather/client
# or
pnpm add @feather/client
```

## Quick Start

```typescript
import { createClient } from '@feather/client';

// Create a client
const client = createClient({
  baseUrl: 'http://localhost:8080',
});

// Get features for an entity
const features = await client.getFeatures('user:123', ['age', 'country']);
console.log(features);
// { entityId: 'user:123', features: { age: 25, country: 'US' } }

// Store features
await client.putFeatures('user:123', {
  age: 25,
  country: 'US',
  lastLogin: new Date().toISOString(),
});

// Batch get
const batch = await client.batchGet(['user:1', 'user:2', 'user:3']);
```

## Vector Search

```typescript
// Create a vector index
await client.vectors.createIndex('embeddings', 128, 'cosine');

// Upsert vectors
await client.vectors.upsert('embeddings', [
  { id: 'doc:1', vector: [...], metadata: { title: 'Document 1' } },
  { id: 'doc:2', vector: [...], metadata: { title: 'Document 2' } },
]);

// Search for similar vectors
const results = await client.vectors.search('embeddings', queryVector, 10, {
  includeMetadata: true,
});
console.log(results);
// [{ id: 'doc:1', score: 0.95, metadata: { title: 'Document 1' } }, ...]
```

## Point-in-Time Queries

```typescript
// Get features as they were at a specific time
const historicalFeatures = await client.getFeaturesAsOf(
  'user:123',
  new Date('2024-01-01T00:00:00Z'),
  ['age', 'plan']
);
```

## Aggregations

```typescript
// Get real-time aggregations
const result = await client.getAggregation(
  'user:123',
  'purchase_amount',
  'sum',
  3600 // 1 hour window
);
console.log(result.value);
```

## Configuration

```typescript
const client = createClient({
  // Required: Feather server URL
  baseUrl: 'http://localhost:8080',

  // Optional: Request timeout (default: 30000ms)
  timeout: 5000,

  // Optional: API key for authentication
  apiKey: 'your-api-key',

  // Optional: Additional headers
  headers: {
    'X-Custom-Header': 'value',
  },

  // Optional: Retry configuration
  retry: {
    maxRetries: 3,
    initialDelay: 100,
    maxDelay: 5000,
    multiplier: 2,
  },
});
```

## Error Handling

```typescript
import { NotFoundError, ValidationError, ConnectionError } from '@feather/client';

try {
  await client.getFeatures('unknown-entity');
} catch (error) {
  if (error instanceof NotFoundError) {
    console.log('Entity not found');
  } else if (error instanceof ValidationError) {
    console.log('Invalid request:', error.message);
  } else if (error instanceof ConnectionError) {
    console.log('Connection failed:', error.message);
  }
}
```

## Health Checks

```typescript
// Full health status
const health = await client.health();
console.log(health.status); // 'healthy' | 'unhealthy' | 'degraded'

// Simple readiness check
const isReady = await client.ready();
```

## TypeScript Support

This SDK is written in TypeScript and includes full type definitions:

```typescript
import type {
  Feature,
  FeatureGroup,
  VectorIndex,
  VectorSearchResult,
  HealthStatus,
} from '@feather/client';
```

## Browser Support

This SDK works in both Node.js and browser environments. It uses the standard `fetch` API, which is available in:

- Node.js 18+
- All modern browsers
- Deno
- Bun

## License

Apache 2.0 - see [LICENSE](../../LICENSE) for details.

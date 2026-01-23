---
sidebar_position: 6
title: TypeScript SDK
description: Official TypeScript/JavaScript client for Feather feature store.
---

# TypeScript SDK

Official TypeScript/JavaScript client for Feather feature store. Works in Node.js, browsers, Deno, and Bun.

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

## Feature Operations

### Get Features

Retrieve features for a single entity:

```typescript
// Get specific features
const response = await client.getFeatures('user:123', ['clicks', 'purchases']);
console.log(response.entityId);    // 'user:123'
console.log(response.features);    // { clicks: { value: 42, timestamp: '...' }, ... }

// Get all features (omit feature list)
const allFeatures = await client.getFeatures('user:123');
```

### Store Features

```typescript
// Store with current timestamp
await client.putFeatures('user:123', {
  clicks: 42,
  purchases: 150.0,
  isPremium: true,
  tags: ['vip', 'early-adopter'],
});

// Store with explicit timestamp
await client.putFeatures('user:123', {
  clicks: 42,
}, new Date('2024-01-15T10:30:00Z'));
```

### Batch Operations

Retrieve features for multiple entities efficiently:

```typescript
const batch = await client.batchGet(
  ['user:123', 'user:456', 'user:789'],
  ['clicks', 'purchases']
);

for (const entity of batch.results) {
  console.log(`${entity.entityId}:`, entity.features);
}
```

## Point-in-Time Queries

Retrieve features as they existed at a specific timestamp:

```typescript
// Get historical feature values
const historicalFeatures = await client.getFeaturesAsOf(
  'user:123',
  new Date('2024-01-01T00:00:00Z'),
  ['age', 'plan']
);

console.log('Age at that time:', historicalFeatures.features.age?.value);
```

This is essential for generating training data without data leakage.

## Aggregations

Get real-time sliding window aggregations:

```typescript
const result = await client.getAggregation(
  'user:123',
  'purchase_amount',
  'sum',
  3600 // 1 hour window in seconds
);
console.log('Total purchases:', result.value);
```

Available aggregation functions:

| Function | Description |
|----------|-------------|
| `'count'` | Number of values |
| `'sum'` | Sum of values |
| `'avg'` | Average of values |
| `'min'` | Minimum value |
| `'max'` | Maximum value |

## Vector Search

Feather includes built-in vector similarity search:

### Create an Index

```typescript
await client.vectors.createIndex('embeddings', 128, 'cosine');
```

Distance types: `'cosine'`, `'euclidean'`, `'dotproduct'`

### Upsert Vectors

```typescript
await client.vectors.upsert('embeddings', [
  {
    id: 'doc:1',
    vector: [0.1, 0.2, /* ... 128 dimensions */],
    metadata: { title: 'Document 1', category: 'tech' }
  },
  {
    id: 'doc:2',
    vector: [0.3, 0.4, /* ... */],
    metadata: { title: 'Document 2', category: 'science' }
  },
]);
```

### Search for Similar Vectors

```typescript
const results = await client.vectors.search('embeddings', queryVector, 10, {
  includeMetadata: true,
  includeVector: false,
});

for (const result of results) {
  console.log(`${result.id}: score=${result.score}`);
  console.log('  Title:', result.metadata?.title);
}
```

### Manage Indexes

```typescript
// List all indexes
const indexes = await client.vectors.listIndexes();
for (const index of indexes) {
  console.log(`${index.name}: ${index.dimension} dimensions, ${index.count} vectors`);
}

// Get index info
const info = await client.vectors.getIndex('embeddings');

// Get a specific vector
const vector = await client.vectors.get('embeddings', 'doc:1');

// Delete a vector
await client.vectors.delete('embeddings', 'doc:1');

// Delete an index
await client.vectors.deleteIndex('embeddings');
```

## Schema Operations

Work with feature groups:

```typescript
// List all feature groups
const groups = await client.listFeatureGroups();
for (const group of groups) {
  console.log(`Group: ${group.name}`);
  for (const feature of group.features) {
    console.log(`  - ${feature.name}: ${feature.dataType}`);
  }
}

// Get a specific feature group
const userFeatures = await client.getFeatureGroup('user_features');
```

## Configuration

Full configuration options:

```typescript
const client = createClient({
  // Required: Feather server URL
  baseUrl: 'http://localhost:8080',

  // Optional: Request timeout in milliseconds (default: 30000)
  timeout: 5000,

  // Optional: API key for authentication
  apiKey: 'your-api-key',

  // Optional: Additional headers
  headers: {
    'X-Custom-Header': 'value',
    'X-Request-Source': 'my-service',
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

The SDK provides typed errors:

```typescript
import {
  FeatherError,
  NotFoundError,
  ValidationError,
  AuthenticationError,
  RateLimitError,
  ConnectionError,
} from '@feather/client';

try {
  await client.getFeatures('unknown-entity');
} catch (error) {
  if (error instanceof NotFoundError) {
    console.log('Entity not found:', error.message);
  } else if (error instanceof ValidationError) {
    console.log('Invalid request:', error.message);
  } else if (error instanceof AuthenticationError) {
    console.log('Authentication failed:', error.message);
  } else if (error instanceof RateLimitError) {
    console.log('Rate limited, retry after:', error.retryAfter);
  } else if (error instanceof ConnectionError) {
    console.log('Connection failed:', error.message);
  } else if (error instanceof FeatherError) {
    console.log(`Error ${error.statusCode}:`, error.message);
  }
}
```

### Error Properties

```typescript
try {
  await client.getFeatures('user:123');
} catch (error) {
  if (error instanceof FeatherError) {
    console.log('Status code:', error.statusCode);
    console.log('Error code:', error.code);  // e.g., 'NOT_FOUND'
    console.log('Message:', error.message);
    console.log('Request ID:', error.requestId);
    console.log('Retryable:', error.isRetryable);
  }
}
```

## Health Checks

Monitor Feather server health:

```typescript
// Full health status
const health = await client.health();
console.log('Status:', health.status);  // 'healthy' | 'unhealthy' | 'degraded'

for (const [name, component] of Object.entries(health.components)) {
  console.log(`  ${name}: ${component.status}`);
}

// Simple readiness check (returns boolean)
const isReady = await client.ready();
if (!isReady) {
  console.log('Server not ready');
}

// Liveness check
const isLive = await client.live();
```

## TypeScript Support

Full TypeScript definitions are included:

```typescript
import type {
  FeatherClient,
  Feature,
  FeatureValue,
  FeatureGroup,
  VectorIndex,
  VectorRecord,
  VectorSearchResult,
  HealthStatus,
  AggFunction,
  DistanceType,
} from '@feather/client';

// Type-safe feature access
const features = await client.getFeatures('user:123', ['age', 'name']);
const age: FeatureValue | undefined = features.features.age;
if (age) {
  console.log('Value:', age.value);      // any
  console.log('Timestamp:', age.timestamp);  // string
}
```

### Generic Types

```typescript
// Define your feature types
interface UserFeatures {
  age: number;
  name: string;
  premium: boolean;
}

// Use with type assertion
const features = await client.getFeatures('user:123', ['age', 'name']);
const age = features.features.age?.value as number;
```

## Browser Support

This SDK works in browser environments using the standard `fetch` API:

```typescript
// In a browser application
import { createClient } from '@feather/client';

const client = createClient({
  baseUrl: 'https://feather.yourapp.com',
  headers: {
    'Authorization': `Bearer ${userToken}`,
  },
});

// Use in your UI
async function loadUserFeatures(userId: string) {
  const features = await client.getFeatures(`user:${userId}`, ['name', 'premium']);
  return features;
}
```

### CORS Configuration

If calling from a browser, ensure your Feather server allows CORS:

```yaml
server:
  http:
    cors:
      enabled: true
      allowed_origins: ["https://yourapp.com"]
      allowed_methods: ["GET", "POST", "PUT", "DELETE"]
      allowed_headers: ["Content-Type", "Authorization"]
```

## Runtime Support

The SDK works with:

- **Node.js**: 18+ (uses native fetch)
- **Browsers**: All modern browsers (Chrome, Firefox, Safari, Edge)
- **Deno**: Full support
- **Bun**: Full support

For Node.js < 18, you may need a fetch polyfill:

```typescript
import fetch from 'node-fetch';

const client = createClient({
  baseUrl: 'http://localhost:8080',
  fetch: fetch as unknown as typeof globalThis.fetch,
});
```

## Complete Example

```typescript
import { createClient, NotFoundError } from '@feather/client';

async function main() {
  // Create client
  const client = createClient({
    baseUrl: 'http://localhost:8080',
    timeout: 10000,
  });

  try {
    // Check server health
    const health = await client.health();
    console.log('Server status:', health.status);

    // Store features
    await client.putFeatures('user:123', {
      name: 'Alice',
      age: 30,
      premium: true,
      balance: 150.50,
      tags: ['vip', 'early-adopter'],
    });
    console.log('Features stored');

    // Get features
    const features = await client.getFeatures('user:123', ['name', 'age']);
    console.log('Name:', features.features.name?.value);
    console.log('Age:', features.features.age?.value);

    // Point-in-time query
    const oneHourAgo = new Date(Date.now() - 3600 * 1000);
    const historical = await client.getFeaturesAsOf('user:123', oneHourAgo, ['balance']);
    console.log('Balance 1 hour ago:', historical.features.balance?.value);

    // Batch get
    const batch = await client.batchGet(
      ['user:123', 'user:456'],
      ['name', 'premium']
    );
    for (const entity of batch.results) {
      console.log(`${entity.entityId}:`, entity.features);
    }

    // Aggregation
    const agg = await client.getAggregation('user:123', 'purchases', 'sum', 3600);
    console.log('Total purchases (1h):', agg.value);

    // Vector search
    await client.vectors.createIndex('embeddings', 128, 'cosine');

    await client.vectors.upsert('embeddings', [
      {
        id: 'doc1',
        vector: new Array(128).fill(0.1),
        metadata: { title: 'Document 1' },
      },
    ]);

    const results = await client.vectors.search(
      'embeddings',
      new Array(128).fill(0.1),
      5,
      { includeMetadata: true }
    );
    for (const result of results) {
      console.log(`Found: ${result.id} (score: ${result.score.toFixed(4)})`);
    }

    console.log('Done!');

  } catch (error) {
    if (error instanceof NotFoundError) {
      console.error('Entity not found:', error.message);
    } else {
      console.error('Error:', error);
    }
  }
}

main();
```

## React Hook Example

```typescript
import { useState, useEffect } from 'react';
import { createClient, FeatherError } from '@feather/client';

const client = createClient({ baseUrl: 'http://localhost:8080' });

function useFeatures(entityId: string, featureNames: string[]) {
  const [features, setFeatures] = useState<Record<string, any> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<FeatherError | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function fetchFeatures() {
      try {
        setLoading(true);
        const response = await client.getFeatures(entityId, featureNames);
        if (!cancelled) {
          setFeatures(response.features);
          setError(null);
        }
      } catch (err) {
        if (!cancelled && err instanceof FeatherError) {
          setError(err);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    fetchFeatures();
    return () => { cancelled = true; };
  }, [entityId, featureNames.join(',')]);

  return { features, loading, error };
}

// Usage in a component
function UserProfile({ userId }: { userId: string }) {
  const { features, loading, error } = useFeatures(
    `user:${userId}`,
    ['name', 'premium', 'balance']
  );

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div>
      <h1>{features?.name?.value}</h1>
      <p>Premium: {features?.premium?.value ? 'Yes' : 'No'}</p>
      <p>Balance: ${features?.balance?.value}</p>
    </div>
  );
}
```

## Related Documentation

- [API Reference](/docs/api-reference) - Complete HTTP/gRPC API documentation
- [Python SDK](/docs/sdks/python) - Python client documentation
- [Go SDK](/docs/sdks/go) - Go client documentation
- [Java/Kotlin SDK](/docs/sdks/java) - Java/Kotlin client documentation
- [Configuration](/docs/configuration) - Server configuration options

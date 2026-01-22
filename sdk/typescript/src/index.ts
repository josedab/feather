/**
 * Feather Feature Store TypeScript/JavaScript SDK
 *
 * @example
 * ```typescript
 * import { createClient } from '@feather/client';
 *
 * const client = createClient({
 *   baseUrl: 'http://localhost:8080',
 * });
 *
 * // Get features
 * const features = await client.getFeatures('user:123', ['age', 'country']);
 *
 * // Store features
 * await client.putFeatures('user:123', { age: 25, country: 'US' });
 *
 * // Vector search
 * const results = await client.vectors.search('embeddings', [0.1, 0.2, ...], 10);
 * ```
 */

export { FeatherClient, VectorClient, createClient } from './client';

export {
  // Feature types
  Feature,
  FeatureValue,
  FeatureGroup,
  FeatureSpec,
  DataType,
  EntityFeatures,
  GetFeaturesRequest,
  GetFeaturesResponse,
  PutFeaturesRequest,
  BatchGetRequest,
  BatchGetResponse,

  // Vector types
  VectorIndex,
  DistanceType,
  VectorRecord,
  CreateIndexRequest,
  UpsertVectorsRequest,
  UpsertResponse,
  SearchVectorsRequest,
  VectorSearchResult,
  VectorSearchResponse,

  // Health types
  HealthStatus,
  ComponentHealth,

  // Aggregation types
  AggFunction,
  AggregationRequest,
  AggregationResponse,

  // Error types
  FeatherError,
  NotFoundError,
  ValidationError,
  ConnectionError,

  // Client options
  ClientOptions,
  RetryOptions,
} from './types';

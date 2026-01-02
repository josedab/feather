/**
 * Feather Feature Store TypeScript SDK Types
 */

// Feature Types

export interface Feature {
  name: string;
  value: FeatureValue;
  timestamp?: string;
  version?: number;
}

export type FeatureValue =
  | string
  | number
  | boolean
  | string[]
  | number[]
  | Record<string, unknown>;

export interface FeatureGroup {
  name: string;
  description?: string;
  entityType: string;
  features: FeatureSpec[];
  ttl?: number;
  tags?: Record<string, string>;
}

export interface FeatureSpec {
  name: string;
  dataType: DataType;
  defaultValue?: FeatureValue;
}

export type DataType =
  | 'string'
  | 'int64'
  | 'float64'
  | 'bool'
  | 'bytes'
  | 'timestamp'
  | 'string_list'
  | 'int64_list'
  | 'float64_list'
  | 'map';

export interface EntityFeatures {
  entityId: string;
  features: Record<string, FeatureValue>;
  timestamp?: string;
}

export interface GetFeaturesRequest {
  entityId: string;
  featureNames?: string[];
  asOf?: string;
}

export interface GetFeaturesResponse {
  entityId: string;
  features: Record<string, FeatureValue>;
  metadata?: Record<string, unknown>;
}

export interface PutFeaturesRequest {
  entityId: string;
  features: Record<string, FeatureValue>;
  timestamp?: string;
}

export interface BatchGetRequest {
  entities: string[];
  featureNames?: string[];
}

export interface BatchGetResponse {
  results: EntityFeatures[];
  errors?: Record<string, string>;
}

// Vector Types

export interface VectorIndex {
  name: string;
  dimension: number;
  distanceType: DistanceType;
  vectorCount: number;
  createdAt: string;
}

export type DistanceType = 'cosine' | 'euclidean' | 'dot_product';

export interface VectorRecord {
  id: string;
  vector: number[];
  metadata?: Record<string, unknown>;
}

export interface CreateIndexRequest {
  name: string;
  dimension: number;
  distanceType?: DistanceType;
}

export interface UpsertVectorsRequest {
  vectors: VectorRecord[];
}

export interface UpsertResponse {
  upsertedCount: number;
}

export interface SearchVectorsRequest {
  vector: number[];
  topK?: number;
  filter?: Record<string, unknown>;
  includeMetadata?: boolean;
  includeVector?: boolean;
}

export interface VectorSearchResult {
  id: string;
  score: number;
  vector?: number[];
  metadata?: Record<string, unknown>;
}

export interface VectorSearchResponse {
  results: VectorSearchResult[];
  took: number;
}

// Health Types

export interface HealthStatus {
  status: 'healthy' | 'unhealthy' | 'degraded';
  components: Record<string, ComponentHealth>;
  version?: string;
  uptime?: number;
}

export interface ComponentHealth {
  status: 'healthy' | 'unhealthy';
  message?: string;
  latency?: number;
}

// Aggregation Types

export type AggFunction = 'count' | 'sum' | 'avg' | 'min' | 'max' | 'last';

export interface AggregationRequest {
  entityId: string;
  feature: string;
  function: AggFunction;
  windowSeconds: number;
}

export interface AggregationResponse {
  entityId: string;
  feature: string;
  function: AggFunction;
  value: number;
  windowStart: string;
  windowEnd: string;
}

// Error Types

export class FeatherError extends Error {
  constructor(
    message: string,
    public statusCode?: number,
    public code?: string
  ) {
    super(message);
    this.name = 'FeatherError';
  }
}

export class NotFoundError extends FeatherError {
  constructor(message: string) {
    super(message, 404, 'NOT_FOUND');
    this.name = 'NotFoundError';
  }
}

export class ValidationError extends FeatherError {
  constructor(message: string) {
    super(message, 400, 'VALIDATION_ERROR');
    this.name = 'ValidationError';
  }
}

export class ConnectionError extends FeatherError {
  constructor(message: string) {
    super(message, undefined, 'CONNECTION_ERROR');
    this.name = 'ConnectionError';
  }
}

// Client Options

export interface ClientOptions {
  /** Base URL of the Feather server */
  baseUrl: string;
  /** Request timeout in milliseconds */
  timeout?: number;
  /** Additional headers to include in requests */
  headers?: Record<string, string>;
  /** API key for authentication */
  apiKey?: string;
  /** Retry configuration */
  retry?: RetryOptions;
}

export interface RetryOptions {
  /** Maximum number of retries */
  maxRetries?: number;
  /** Initial delay between retries in ms */
  initialDelay?: number;
  /** Maximum delay between retries in ms */
  maxDelay?: number;
  /** Backoff multiplier */
  multiplier?: number;
}

/**
 * Feather Feature Store TypeScript Client
 */

import {
  AggFunction,
  AggregationResponse,
  BatchGetResponse,
  ClientOptions,
  ConnectionError,
  CreateIndexRequest,
  EntityFeatures,
  FeatherError,
  FeatureGroup,
  FeatureValue,
  GetFeaturesResponse,
  HealthStatus,
  NotFoundError,
  RetryOptions,
  UpsertResponse,
  ValidationError,
  VectorIndex,
  VectorRecord,
  VectorSearchResponse,
  VectorSearchResult,
} from './types';

const DEFAULT_TIMEOUT = 30000;
const DEFAULT_RETRY: RetryOptions = {
  maxRetries: 3,
  initialDelay: 100,
  maxDelay: 5000,
  multiplier: 2,
};

/**
 * VectorClient handles vector similarity search operations.
 */
export class VectorClient {
  constructor(
    private client: FeatherClient,
    private baseUrl: string
  ) {}

  /**
   * List all vector indexes.
   */
  async listIndexes(): Promise<string[]> {
    const response = await this.client.request<{ indexes: string[] }>(
      'GET',
      `${this.baseUrl}/v1/vectors`
    );
    return response.indexes || [];
  }

  /**
   * Create a new vector index.
   */
  async createIndex(
    name: string,
    dimension: number,
    distanceType: 'cosine' | 'euclidean' | 'dot_product' = 'cosine'
  ): Promise<VectorIndex> {
    const request: CreateIndexRequest = {
      name,
      dimension,
      distanceType,
    };
    return this.client.request<VectorIndex>(
      'POST',
      `${this.baseUrl}/v1/vectors`,
      request
    );
  }

  /**
   * Get information about a vector index.
   */
  async getIndex(name: string): Promise<VectorIndex> {
    return this.client.request<VectorIndex>(
      'GET',
      `${this.baseUrl}/v1/vectors/${name}`
    );
  }

  /**
   * Delete a vector index.
   */
  async deleteIndex(name: string): Promise<void> {
    await this.client.request('DELETE', `${this.baseUrl}/v1/vectors/${name}`);
  }

  /**
   * Upsert vectors into an index.
   */
  async upsert(index: string, vectors: VectorRecord[]): Promise<number> {
    const response = await this.client.request<UpsertResponse>(
      'POST',
      `${this.baseUrl}/v1/vectors/${index}/upsert`,
      { vectors }
    );
    return response.upsertedCount;
  }

  /**
   * Search for similar vectors.
   */
  async search(
    index: string,
    vector: number[],
    topK: number = 10,
    options?: {
      filter?: Record<string, unknown>;
      includeMetadata?: boolean;
      includeVector?: boolean;
    }
  ): Promise<VectorSearchResult[]> {
    const response = await this.client.request<VectorSearchResponse>(
      'POST',
      `${this.baseUrl}/v1/vectors/${index}/search`,
      {
        vector,
        topK,
        filter: options?.filter,
        includeMetadata: options?.includeMetadata ?? true,
        includeVector: options?.includeVector ?? false,
      }
    );
    return response.results;
  }

  /**
   * Get a vector by ID.
   */
  async get(index: string, id: string): Promise<VectorRecord | null> {
    try {
      return await this.client.request<VectorRecord>(
        'GET',
        `${this.baseUrl}/v1/vectors/${index}/${id}`
      );
    } catch (error) {
      if (error instanceof NotFoundError) {
        return null;
      }
      throw error;
    }
  }

  /**
   * Delete a vector by ID.
   */
  async delete(index: string, id: string): Promise<void> {
    await this.client.request(
      'DELETE',
      `${this.baseUrl}/v1/vectors/${index}/${id}`
    );
  }
}

/**
 * FeatherClient is the main client for the Feather Feature Store.
 */
export class FeatherClient {
  private baseUrl: string;
  private timeout: number;
  private headers: Record<string, string>;
  private retry: RetryOptions;
  public vectors: VectorClient;

  constructor(options: ClientOptions) {
    this.baseUrl = options.baseUrl.replace(/\/$/, '');
    this.timeout = options.timeout ?? DEFAULT_TIMEOUT;
    this.headers = {
      'Content-Type': 'application/json',
      ...options.headers,
    };
    if (options.apiKey) {
      this.headers['Authorization'] = `Bearer ${options.apiKey}`;
    }
    this.retry = { ...DEFAULT_RETRY, ...options.retry };
    this.vectors = new VectorClient(this, this.baseUrl);
  }

  /**
   * Get features for an entity.
   */
  async getFeatures(
    entityId: string,
    featureNames?: string[]
  ): Promise<GetFeaturesResponse> {
    const params = new URLSearchParams({ entity: entityId });
    if (featureNames) {
      featureNames.forEach((f) => params.append('feature', f));
    }
    return this.request<GetFeaturesResponse>(
      'GET',
      `${this.baseUrl}/v1/features?${params}`
    );
  }

  /**
   * Store features for an entity.
   */
  async putFeatures(
    entityId: string,
    features: Record<string, FeatureValue>,
    timestamp?: string
  ): Promise<void> {
    await this.request('POST', `${this.baseUrl}/v1/features`, {
      entity_id: entityId,
      features,
      timestamp,
    });
  }

  /**
   * Get features for multiple entities.
   */
  async batchGet(
    entityIds: string[],
    featureNames?: string[]
  ): Promise<BatchGetResponse> {
    return this.request<BatchGetResponse>(
      'POST',
      `${this.baseUrl}/v1/features/batch`,
      {
        entities: entityIds,
        features: featureNames,
      }
    );
  }

  /**
   * Get features at a specific point in time.
   */
  async getFeaturesAsOf(
    entityId: string,
    asOf: Date | string,
    featureNames?: string[]
  ): Promise<GetFeaturesResponse> {
    const timestamp = asOf instanceof Date ? asOf.toISOString() : asOf;
    const params = new URLSearchParams({
      entity: entityId,
      as_of: timestamp,
    });
    if (featureNames) {
      featureNames.forEach((f) => params.append('feature', f));
    }
    return this.request<GetFeaturesResponse>(
      'GET',
      `${this.baseUrl}/v1/features/history?${params}`
    );
  }

  /**
   * Get aggregated feature value.
   */
  async getAggregation(
    entityId: string,
    feature: string,
    fn: AggFunction,
    windowSeconds: number
  ): Promise<AggregationResponse> {
    const params = new URLSearchParams({
      entity: entityId,
      feature,
      function: fn,
      window: windowSeconds.toString(),
    });
    return this.request<AggregationResponse>(
      'GET',
      `${this.baseUrl}/v1/aggregation?${params}`
    );
  }

  /**
   * List feature groups.
   */
  async listFeatureGroups(): Promise<FeatureGroup[]> {
    const response = await this.request<{ groups: FeatureGroup[] }>(
      'GET',
      `${this.baseUrl}/v1/schema/groups`
    );
    return response.groups || [];
  }

  /**
   * Get a feature group by name.
   */
  async getFeatureGroup(name: string): Promise<FeatureGroup | null> {
    try {
      return await this.request<FeatureGroup>(
        'GET',
        `${this.baseUrl}/v1/schema/groups/${name}`
      );
    } catch (error) {
      if (error instanceof NotFoundError) {
        return null;
      }
      throw error;
    }
  }

  /**
   * Check server health.
   */
  async health(): Promise<HealthStatus> {
    return this.request<HealthStatus>('GET', `${this.baseUrl}/health`);
  }

  /**
   * Check if server is ready.
   */
  async ready(): Promise<boolean> {
    try {
      await this.request('GET', `${this.baseUrl}/ready`);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Make an HTTP request with retry logic.
   */
  async request<T>(
    method: string,
    url: string,
    body?: unknown
  ): Promise<T> {
    let lastError: Error | null = null;
    let delay = this.retry.initialDelay ?? 100;

    for (let attempt = 0; attempt <= (this.retry.maxRetries ?? 3); attempt++) {
      try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), this.timeout);

        const options: RequestInit = {
          method,
          headers: this.headers,
          signal: controller.signal,
        };

        if (body) {
          options.body = JSON.stringify(body);
        }

        const response = await fetch(url, options);
        clearTimeout(timeoutId);

        if (!response.ok) {
          const errorBody = await response.text();
          let errorMessage = errorBody;
          try {
            const parsed = JSON.parse(errorBody);
            errorMessage = parsed.error || parsed.message || errorBody;
          } catch {
            // Use raw error body
          }

          if (response.status === 404) {
            throw new NotFoundError(errorMessage);
          }
          if (response.status === 400) {
            throw new ValidationError(errorMessage);
          }

          // Retry on 5xx errors
          if (response.status >= 500 && attempt < (this.retry.maxRetries ?? 3)) {
            lastError = new FeatherError(errorMessage, response.status);
            await this.sleep(delay);
            delay = Math.min(
              delay * (this.retry.multiplier ?? 2),
              this.retry.maxDelay ?? 5000
            );
            continue;
          }

          throw new FeatherError(errorMessage, response.status);
        }

        const text = await response.text();
        if (!text) {
          return {} as T;
        }
        return JSON.parse(text) as T;
      } catch (error) {
        if (error instanceof FeatherError) {
          throw error;
        }

        if (error instanceof DOMException && error.name === 'AbortError') {
          lastError = new ConnectionError('Request timeout');
        } else if (error instanceof TypeError) {
          lastError = new ConnectionError(`Connection failed: ${error.message}`);
        } else {
          lastError = error as Error;
        }

        if (attempt < (this.retry.maxRetries ?? 3)) {
          await this.sleep(delay);
          delay = Math.min(
            delay * (this.retry.multiplier ?? 2),
            this.retry.maxDelay ?? 5000
          );
          continue;
        }
      }
    }

    throw lastError || new ConnectionError('Request failed after retries');
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}

/**
 * Create a new Feather client.
 */
export function createClient(options: ClientOptions): FeatherClient {
  return new FeatherClient(options);
}

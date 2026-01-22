import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { FeatherClient, createClient } from './client';
import { NotFoundError, ValidationError, ConnectionError } from './types';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('FeatherClient', () => {
  let client: FeatherClient;

  beforeEach(() => {
    mockFetch.mockReset();
    client = createClient({
      baseUrl: 'http://localhost:8080',
      timeout: 5000,
      retry: { maxRetries: 0 }, // Disable retries for tests
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('getFeatures', () => {
    it('should fetch features for an entity', async () => {
      const mockResponse = {
        entityId: 'user:123',
        features: { age: 25, country: 'US' },
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify(mockResponse)),
      });

      const result = await client.getFeatures('user:123');

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/v1/features?entity=user%3A123',
        expect.objectContaining({
          method: 'GET',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        })
      );
      expect(result).toEqual(mockResponse);
    });

    it('should include feature names in query', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify({})),
      });

      await client.getFeatures('user:123', ['age', 'country']);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('feature=age'),
        expect.any(Object)
      );
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('feature=country'),
        expect.any(Object)
      );
    });
  });

  describe('putFeatures', () => {
    it('should store features for an entity', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(''),
      });

      await client.putFeatures('user:123', { age: 25, country: 'US' });

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/v1/features',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            entity_id: 'user:123',
            features: { age: 25, country: 'US' },
            timestamp: undefined,
          }),
        })
      );
    });
  });

  describe('batchGet', () => {
    it('should fetch features for multiple entities', async () => {
      const mockResponse = {
        results: [
          { entityId: 'user:1', features: { age: 20 } },
          { entityId: 'user:2', features: { age: 30 } },
        ],
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify(mockResponse)),
      });

      const result = await client.batchGet(['user:1', 'user:2']);

      expect(result).toEqual(mockResponse);
    });
  });

  describe('getFeaturesAsOf', () => {
    it('should fetch historical features', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify({})),
      });

      const date = new Date('2024-01-01T00:00:00Z');
      await client.getFeaturesAsOf('user:123', date);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('as_of=2024-01-01T00%3A00%3A00.000Z'),
        expect.any(Object)
      );
    });
  });

  describe('health', () => {
    it('should return health status', async () => {
      const mockResponse = {
        status: 'healthy',
        components: { storage: { status: 'healthy' } },
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify(mockResponse)),
      });

      const result = await client.health();

      expect(result.status).toBe('healthy');
    });
  });

  describe('ready', () => {
    it('should return true when server is ready', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(''),
      });

      const result = await client.ready();

      expect(result).toBe(true);
    });

    it('should return false when server is not ready', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 503,
        text: () => Promise.resolve('Service Unavailable'),
      });

      const result = await client.ready();

      expect(result).toBe(false);
    });
  });

  describe('error handling', () => {
    it('should throw NotFoundError for 404', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: () => Promise.resolve('{"error": "Entity not found"}'),
      });

      await expect(client.getFeatures('unknown')).rejects.toThrow(NotFoundError);
    });

    it('should throw ValidationError for 400', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: () => Promise.resolve('{"error": "Invalid request"}'),
      });

      await expect(client.putFeatures('', {})).rejects.toThrow(ValidationError);
    });
  });
});

describe('VectorClient', () => {
  let client: FeatherClient;

  beforeEach(() => {
    mockFetch.mockReset();
    client = createClient({
      baseUrl: 'http://localhost:8080',
      retry: { maxRetries: 0 },
    });
  });

  describe('listIndexes', () => {
    it('should list all vector indexes', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify({ indexes: ['index1', 'index2'] })),
      });

      const result = await client.vectors.listIndexes();

      expect(result).toEqual(['index1', 'index2']);
    });
  });

  describe('createIndex', () => {
    it('should create a new vector index', async () => {
      const mockResponse = {
        name: 'test-index',
        dimension: 128,
        distanceType: 'cosine',
        vectorCount: 0,
        createdAt: '2024-01-01T00:00:00Z',
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify(mockResponse)),
      });

      const result = await client.vectors.createIndex('test-index', 128);

      expect(result).toEqual(mockResponse);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/v1/vectors',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            name: 'test-index',
            dimension: 128,
            distanceType: 'cosine',
          }),
        })
      );
    });
  });

  describe('search', () => {
    it('should search for similar vectors', async () => {
      const mockResponse = {
        results: [
          { id: 'vec1', score: 0.95, metadata: { label: 'test' } },
          { id: 'vec2', score: 0.87 },
        ],
        took: 5,
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify(mockResponse)),
      });

      const result = await client.vectors.search('index', [0.1, 0.2, 0.3], 10);

      expect(result).toEqual(mockResponse.results);
    });
  });

  describe('upsert', () => {
    it('should upsert vectors', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify({ upsertedCount: 2 })),
      });

      const vectors = [
        { id: 'v1', vector: [0.1, 0.2] },
        { id: 'v2', vector: [0.3, 0.4], metadata: { label: 'test' } },
      ];

      const result = await client.vectors.upsert('index', vectors);

      expect(result).toBe(2);
    });
  });

  describe('get', () => {
    it('should return null for non-existent vector', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: () => Promise.resolve('{"error": "Vector not found"}'),
      });

      const result = await client.vectors.get('index', 'nonexistent');

      expect(result).toBeNull();
    });
  });
});

describe('createClient', () => {
  it('should create a client with default options', () => {
    const client = createClient({ baseUrl: 'http://localhost:8080' });

    expect(client).toBeInstanceOf(FeatherClient);
    expect(client.vectors).toBeDefined();
  });

  it('should include API key in headers', async () => {
    const client = createClient({
      baseUrl: 'http://localhost:8080',
      apiKey: 'test-api-key',
      retry: { maxRetries: 0 },
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      text: () => Promise.resolve('{}'),
    });

    await client.health();

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer test-api-key',
        }),
      })
    );
  });
});

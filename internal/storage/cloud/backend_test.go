package cloud

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// MockDynamoDBClient is a mock implementation for testing.
type MockDynamoDBClient struct {
	mu    sync.RWMutex
	items map[string]map[string]interface{}
}

func NewMockDynamoDBClient() *MockDynamoDBClient {
	return &MockDynamoDBClient{
		items: make(map[string]map[string]interface{}),
	}
}

func (m *MockDynamoDBClient) GetItem(ctx context.Context, input *GetItemInput) (*GetItemOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pk, _ := input.Key["pk"].(string)
	item, ok := m.items[pk]
	if !ok {
		return &GetItemOutput{}, nil
	}

	return &GetItemOutput{Item: item}, nil
}

func (m *MockDynamoDBClient) PutItem(ctx context.Context, input *PutItemInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pk, _ := input.Item["pk"].(string)
	m.items[pk] = input.Item
	return nil
}

func (m *MockDynamoDBClient) DeleteItem(ctx context.Context, input *DeleteItemInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pk, _ := input.Key["pk"].(string)
	delete(m.items, pk)
	return nil
}

func (m *MockDynamoDBClient) BatchGetItem(ctx context.Context, input *BatchGetItemInput) (*BatchGetItemOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	responses := make(map[string][]map[string]interface{})

	for tableName, keys := range input.RequestItems {
		items := make([]map[string]interface{}, 0, len(keys.Keys))
		for _, key := range keys.Keys {
			pk, _ := key["pk"].(string)
			if item, ok := m.items[pk]; ok {
				items = append(items, item)
			}
		}
		responses[tableName] = items
	}

	return &BatchGetItemOutput{Responses: responses}, nil
}

func (m *MockDynamoDBClient) BatchWriteItem(ctx context.Context, input *BatchWriteItemInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, requests := range input.RequestItems {
		for _, req := range requests {
			if req.PutRequest != nil {
				pk, _ := req.PutRequest.Item["pk"].(string)
				m.items[pk] = req.PutRequest.Item
			}
			if req.DeleteRequest != nil {
				pk, _ := req.DeleteRequest.Key["pk"].(string)
				delete(m.items, pk)
			}
		}
	}

	return nil
}

func (m *MockDynamoDBClient) Query(ctx context.Context, input *QueryInput) (*QueryOutput, error) {
	return &QueryOutput{Items: []map[string]interface{}{}}, nil
}

func (m *MockDynamoDBClient) Scan(ctx context.Context, input *ScanInput) (*ScanOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]map[string]interface{}, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, item)
		if input.Limit > 0 && len(items) >= input.Limit {
			break
		}
	}

	return &ScanOutput{Items: items}, nil
}

func TestDynamoDBBackendBasic(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = false

	var err error
	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Test Put
	features := map[string]*domain.FeatureValue{
		"click_count": {Value: 42, Timestamp: time.Now().UnixNano()},
		"last_seen":   {Value: "2024-01-15", Timestamp: time.Now().UnixNano()},
	}

	if err = backend.Put(ctx, "user:123", features); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Test Get
	var getErr error
	var retrieved map[string]*domain.FeatureValue
	retrieved, getErr = backend.Get(ctx, "user:123", []string{"click_count"})
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}

	if _, ok := retrieved["click_count"]; !ok {
		t.Error("expected click_count feature")
	}

	// Test Delete
	if err = backend.Delete(ctx, "user:123"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = backend.Get(ctx, "user:123", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected ErrNotFound after delete")
	}
}

func TestDynamoDBBackendBatch(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = false

	var err error
	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Batch put
	updates := map[string]map[string]*domain.FeatureValue{
		"user:1": {"score": {Value: 100, Timestamp: time.Now().UnixNano()}},
		"user:2": {"score": {Value: 200, Timestamp: time.Now().UnixNano()}},
		"user:3": {"score": {Value: 300, Timestamp: time.Now().UnixNano()}},
	}

	if err = backend.BatchPut(ctx, updates); err != nil {
		t.Fatalf("batch put: %v", err)
	}

	// Batch get
	var batchErr error
	var result map[string]map[string]*domain.FeatureValue
	result, batchErr = backend.BatchGet(ctx, []string{"user:1", "user:2", "user:3"}, nil)
	if batchErr != nil {
		t.Fatalf("batch get: %v", batchErr)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 results, got %d", len(result))
	}
}

func TestDynamoDBBackendStats(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = false

	var err error
	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Perform some operations
	features := map[string]*domain.FeatureValue{
		"test": {Value: 1, Timestamp: time.Now().UnixNano()},
	}

	backend.Put(ctx, "key1", features)
	backend.Get(ctx, "key1", nil)

	stats := backend.Stats()

	if stats.WriteOps < 1 {
		t.Error("expected at least 1 write op")
	}
	if stats.ReadOps < 1 {
		t.Error("expected at least 1 read op")
	}
}

func TestDynamoDBBackendHealth(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()

	var err error
	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	ctx := context.Background()

	if err := backend.Health(ctx); err != nil {
		t.Errorf("health check failed: %v", err)
	}

	backend.Close()

	if err := backend.Health(ctx); !errors.Is(err, ErrBackendClosed) {
		t.Error("expected ErrBackendClosed after close")
	}
}

func TestRetry(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
	}

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		if attempts < 3 {
			return ErrConnectionFailed
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected success after retries, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryExhausted(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries: 2,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Multiplier: 2.0,
	}

	err := Retry(ctx, config, func() error {
		return ErrConnectionFailed
	})

	if !errors.Is(err, ErrConnectionFailed) {
		t.Errorf("expected ErrConnectionFailed, got: %v", err)
	}
}

// MockGCSClient is a mock implementation for testing.
type MockGCSClient struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func NewMockGCSClient() *MockGCSClient {
	return &MockGCSClient{
		objects: make(map[string][]byte),
	}
}

func (m *MockGCSClient) Read(ctx context.Context, bucket, object string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := bucket + "/" + object
	data, ok := m.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return data, nil
}

func (m *MockGCSClient) Write(ctx context.Context, bucket, object string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := bucket + "/" + object
	m.objects[key] = data
	return nil
}

func (m *MockGCSClient) Delete(ctx context.Context, bucket, object string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := bucket + "/" + object
	delete(m.objects, key)
	return nil
}

func (m *MockGCSClient) List(ctx context.Context, bucket, prefix string, maxResults int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	objects := make([]string, 0, len(m.objects))
	fullPrefix := bucket + "/" + prefix

	for key := range m.objects {
		if len(key) >= len(fullPrefix) && key[:len(fullPrefix)] == fullPrefix {
			objects = append(objects, key[len(bucket)+1:])
			if maxResults > 0 && len(objects) >= maxResults {
				break
			}
		}
	}

	return objects, nil
}

func (m *MockGCSClient) Exists(ctx context.Context, bucket, object string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := bucket + "/" + object
	_, ok := m.objects[key]
	return ok, nil
}

func TestGCSBackendBasic(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.EnableCompression = false

	var err error
	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	var getErr error
	var retrieved map[string]*domain.FeatureValue

	// Test Put
	features := map[string]*domain.FeatureValue{
		"click_count": {Value: 42, Timestamp: time.Now().UnixNano()},
	}

	if err = backend.Put(ctx, "user:123", features); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Test Get
	retrieved, getErr = backend.Get(ctx, "user:123", nil)
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}

	if _, ok := retrieved["click_count"]; !ok {
		t.Error("expected click_count feature")
	}

	// Test Delete
	if err = backend.Delete(ctx, "user:123"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestGCSBackendBatch(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.EnableCompression = false

	var err error
	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	var batchErr error
	var result map[string]map[string]*domain.FeatureValue

	// Batch put
	updates := map[string]map[string]*domain.FeatureValue{
		"user:1": {"score": {Value: 100, Timestamp: time.Now().UnixNano()}},
		"user:2": {"score": {Value: 200, Timestamp: time.Now().UnixNano()}},
	}

	if err = backend.BatchPut(ctx, updates); err != nil {
		t.Fatalf("batch put: %v", err)
	}

	// Batch get
	result, batchErr = backend.BatchGet(ctx, []string{"user:1", "user:2"}, nil)
	if batchErr != nil {
		t.Fatalf("batch get: %v", batchErr)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
}

func BenchmarkDynamoDBBackendPut(b *testing.B) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = false

	backend, _ := NewDynamoDBBackend(config, client)
	defer backend.Close()

	ctx := context.Background()
	features := map[string]*domain.FeatureValue{
		"f1": {Value: 1, Timestamp: time.Now().UnixNano()},
		"f2": {Value: 2, Timestamp: time.Now().UnixNano()},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Put(ctx, "key", features)
	}
}

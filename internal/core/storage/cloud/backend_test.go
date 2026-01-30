package cloud

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// MockDynamoDBClient is a mock implementation for testing.
type MockDynamoDBClient struct {
	mu          sync.RWMutex
	items       map[string]map[string]interface{}
	historyItems []map[string]interface{}
}

func NewMockDynamoDBClient() *MockDynamoDBClient {
	return &MockDynamoDBClient{
		items:        make(map[string]map[string]interface{}),
		historyItems: make([]map[string]interface{}, 0),
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
	m.mu.RLock()
	defer m.mu.RUnlock()

	pk, _ := input.ExpressionValues[":pk"].(string)
	ts, _ := input.ExpressionValues[":ts"].(int64)

	var matched []map[string]interface{}
	for _, item := range m.historyItems {
		itemPK, _ := item["pk"].(string)
		itemSK, _ := item["sk"].(int64)
		if itemPK == pk && itemSK <= ts {
			matched = append(matched, item)
		}
	}

	// Sort descending by sk and apply limit
	for i := 0; i < len(matched); i++ {
		for j := i + 1; j < len(matched); j++ {
			si, _ := matched[i]["sk"].(int64)
			sj, _ := matched[j]["sk"].(int64)
			if sj > si {
				matched[i], matched[j] = matched[j], matched[i]
			}
		}
	}

	if input.Limit > 0 && len(matched) > input.Limit {
		matched = matched[:input.Limit]
	}

	return &QueryOutput{Items: matched}, nil
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

// --- DynamoDB Additional Tests ---

func TestDynamoDBGetAsOf(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = false
	config.HistoryEnabled = true
	config.HistoryTableName = "feather-history"

	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	now := time.Now()

	// Seed history via the mock
	features := map[string]*domain.FeatureValue{
		"score": {Value: 100, Timestamp: now.Add(-2 * time.Hour).UnixNano()},
	}
	item := backend.featuresToHistoryItem("user:1", features, now.Add(-2*time.Hour))
	client.mu.Lock()
	client.historyItems = append(client.historyItems, item)
	client.mu.Unlock()

	// Query for a time after the entry
	result, err := backend.GetAsOf(ctx, "user:1", nil, now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("GetAsOf: %v", err)
	}
	if result["score"] == nil {
		t.Error("expected score feature in result")
	}

	// Query before any history should return ErrNotFound
	_, err = backend.GetAsOf(ctx, "user:1", nil, now.Add(-3*time.Hour))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDynamoDBGetAsOf_HistoryDisabled(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.HistoryEnabled = false

	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	_, err = backend.GetAsOf(context.Background(), "user:1", nil, time.Now())
	if err == nil {
		t.Error("expected error when history is disabled")
	}
}

func TestDynamoDBFeaturesToHistoryItem(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = false
	config.HistoryTTL = 24 * time.Hour

	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	ts := time.Now()
	features := map[string]*domain.FeatureValue{
		"clicks": {Value: 42, Timestamp: ts.UnixNano()},
	}

	item := backend.featuresToHistoryItem("user:1", features, ts)

	if item["pk"] != "user:1" {
		t.Errorf("expected pk user:1, got %v", item["pk"])
	}
	if item["sk"] != ts.UnixNano() {
		t.Errorf("expected sk %d, got %v", ts.UnixNano(), item["sk"])
	}
	if item["features"] == nil {
		t.Error("expected features data")
	}
	if item["ttl"] == nil {
		t.Error("expected ttl when HistoryTTL is set")
	}
}

func TestDynamoDBFeaturesToHistoryItem_WithCompression(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = true

	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	features := map[string]*domain.FeatureValue{
		"clicks": {Value: 42, Timestamp: time.Now().UnixNano()},
	}

	item := backend.featuresToHistoryItem("user:1", features, time.Now())

	if item["compressed"] != true {
		t.Error("expected compressed=true when compression is enabled")
	}
}

func TestDynamoDBCompressDecompress(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = true

	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	original := []byte(`{"score": {"value": 42, "timestamp": 1234567890}}`)

	compressed, err := backend.compress(original)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if len(compressed) == 0 {
		t.Error("expected non-empty compressed data")
	}

	decompressed, err := backend.decompress(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(decompressed) != string(original) {
		t.Errorf("roundtrip failed: got %s", decompressed)
	}
}

func TestCreateTableSchema(t *testing.T) {
	schema := CreateTableSchema("test-table")

	if schema["TableName"] != "test-table" {
		t.Errorf("expected TableName=test-table, got %v", schema["TableName"])
	}
	if schema["BillingMode"] != "PAY_PER_REQUEST" {
		t.Errorf("expected BillingMode=PAY_PER_REQUEST, got %v", schema["BillingMode"])
	}
	keySchema, ok := schema["KeySchema"].([]map[string]string)
	if !ok || len(keySchema) != 1 {
		t.Error("expected 1 key schema element")
	}
}

func TestCreateHistoryTableSchema(t *testing.T) {
	schema := CreateHistoryTableSchema("test-history")

	if schema["TableName"] != "test-history" {
		t.Errorf("expected TableName=test-history, got %v", schema["TableName"])
	}
	keySchema, ok := schema["KeySchema"].([]map[string]string)
	if !ok || len(keySchema) != 2 {
		t.Error("expected 2 key schema elements (HASH + RANGE)")
	}
	ttl, ok := schema["TimeToLiveSpecification"].(map[string]interface{})
	if !ok || ttl["Enabled"] != true {
		t.Error("expected TTL to be enabled")
	}
}

func TestParseTimestamp(t *testing.T) {
	now := time.Now()
	nanos := now.UnixNano()

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"int64", nanos, false},
		{"float64", float64(nanos), false},
		{"string_int", "1234567890000000000", false},
		{"string_rfc3339", now.Format(time.RFC3339Nano), false},
		{"time.Time", now, false},
		{"invalid_type", struct{}{}, true},
		{"invalid_string", "not-a-timestamp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// --- GCS Additional Tests ---

func TestGCSBackendGetAsOf(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.EnableCompression = false
	config.HistoryEnabled = true

	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	now := time.Now()

	// Put a feature (will write history too)
	features := map[string]*domain.FeatureValue{
		"clicks": {Value: 42, Timestamp: now.Add(-2 * time.Hour).UnixNano()},
	}
	if err := backend.Put(ctx, "user:1", features); err != nil {
		t.Fatalf("put: %v", err)
	}

	// GetAsOf with a time after the write
	result, err := backend.GetAsOf(ctx, "user:1", nil, now.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("GetAsOf: %v", err)
	}
	if result["clicks"] == nil {
		t.Error("expected clicks feature in result")
	}
}

func TestGCSBackendGetAsOf_HistoryDisabled(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.HistoryEnabled = false

	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	_, err = backend.GetAsOf(context.Background(), "user:1", nil, time.Now())
	if err == nil {
		t.Error("expected error when history is disabled")
	}
}

func TestGCSBackendScan(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.EnableCompression = false

	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Put some entities
	for _, entity := range []string{"user:1", "user:2", "user:3"} {
		features := map[string]*domain.FeatureValue{
			"score": {Value: 100, Timestamp: time.Now().UnixNano()},
		}
		if err := backend.Put(ctx, entity, features); err != nil {
			t.Fatalf("put %s: %v", entity, err)
		}
	}

	keys, err := backend.Scan(ctx, "", 100)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(keys) < 3 {
		t.Errorf("expected at least 3 keys, got %d", len(keys))
	}
}

func TestGCSBackendStats(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.EnableCompression = false

	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Perform some operations
	features := map[string]*domain.FeatureValue{
		"score": {Value: 100, Timestamp: time.Now().UnixNano()},
	}
	_ = backend.Put(ctx, "user:1", features)
	_, _ = backend.Get(ctx, "user:1", nil)

	stats := backend.Stats()
	if stats.WriteOps < 1 {
		t.Errorf("expected WriteOps >= 1, got %d", stats.WriteOps)
	}
	if stats.ReadOps < 1 {
		t.Errorf("expected ReadOps >= 1, got %d", stats.ReadOps)
	}
}

func TestGCSBackendHealth(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()

	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	// Health should succeed for open backend
	if err := backend.Health(context.Background()); err != nil {
		t.Errorf("expected healthy, got %v", err)
	}

	// Health should fail for closed backend
	backend.Close()
	if err := backend.Health(context.Background()); !errors.Is(err, ErrBackendClosed) {
		t.Errorf("expected ErrBackendClosed, got %v", err)
	}
}

func TestGCSCompressDecompress(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.EnableCompression = true

	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	original := []byte(`{"clicks": {"value": 42}}`)

	compressed, err := backend.compress(original)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}

	decompressed, err := backend.decompress(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(decompressed) != string(original) {
		t.Errorf("roundtrip failed: got %s", decompressed)
	}
}

func TestGCSBackendWithCompression(t *testing.T) {
	client := NewMockGCSClient()
	config := DefaultGCSConfig()
	config.EnableCompression = true

	backend, err := NewGCSBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	features := map[string]*domain.FeatureValue{
		"clicks": {Value: 42, Timestamp: time.Now().UnixNano()},
	}

	if err := backend.Put(ctx, "user:1", features); err != nil {
		t.Fatalf("put: %v", err)
	}

	result, err := backend.Get(ctx, "user:1", nil)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if result["clicks"] == nil {
		t.Error("expected clicks feature")
	}
}

func TestDynamoDBBackendWithCompression(t *testing.T) {
	client := NewMockDynamoDBClient()
	config := DefaultDynamoDBConfig()
	config.EnableCompression = true

	backend, err := NewDynamoDBBackend(config, client)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	features := map[string]*domain.FeatureValue{
		"clicks": {Value: 42, Timestamp: time.Now().UnixNano()},
	}

	if err := backend.Put(ctx, "user:1", features); err != nil {
		t.Fatalf("put: %v", err)
	}

	result, err := backend.Get(ctx, "user:1", nil)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if result["clicks"] == nil {
		t.Error("expected clicks feature")
	}
}

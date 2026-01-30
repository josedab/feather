package cloud

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// DynamoDBConfig configures the DynamoDB backend.
type DynamoDBConfig struct {
	BackendConfig

	// TableName is the DynamoDB table name.
	TableName string `json:"table_name" yaml:"table_name"`

	// HistoryTableName is the table for historical data (optional).
	HistoryTableName string `json:"history_table_name" yaml:"history_table_name"`

	// AccessKeyID is the AWS access key (optional, uses default chain if empty).
	AccessKeyID string `json:"access_key_id,omitempty" yaml:"access_key_id,omitempty"`

	// SecretAccessKey is the AWS secret key.
	SecretAccessKey string `json:"secret_access_key,omitempty" yaml:"secret_access_key,omitempty"`

	// ConsistentReads enables strongly consistent reads.
	ConsistentReads bool `json:"consistent_reads" yaml:"consistent_reads"`

	// WriteCapacity is the provisioned write capacity units.
	WriteCapacity int64 `json:"write_capacity" yaml:"write_capacity"`

	// ReadCapacity is the provisioned read capacity units.
	ReadCapacity int64 `json:"read_capacity" yaml:"read_capacity"`

	// DAXEndpoint is the DAX cluster endpoint (optional).
	DAXEndpoint string `json:"dax_endpoint,omitempty" yaml:"dax_endpoint,omitempty"`

	// BatchSize is the max items per batch operation.
	BatchSize int `json:"batch_size" yaml:"batch_size"`
}

// DefaultDynamoDBConfig returns default DynamoDB configuration.
func DefaultDynamoDBConfig() DynamoDBConfig {
	return DynamoDBConfig{
		BackendConfig: DefaultBackendConfig(),
		TableName:     "feather-features",
		BatchSize:     25, // DynamoDB batch limit
	}
}

// DynamoDBBackend implements Backend using AWS DynamoDB.
type DynamoDBBackend struct {
	config      DynamoDBConfig
	client      DynamoDBClient
	retryConfig RetryConfig
	stats       dynamoDBStats
	mu          sync.RWMutex
	closed      bool
}

// DynamoDBClient defines the DynamoDB operations interface.
// This allows for mocking in tests and decouples from the actual AWS SDK.
type DynamoDBClient interface {
	GetItem(ctx context.Context, input *GetItemInput) (*GetItemOutput, error)
	PutItem(ctx context.Context, input *PutItemInput) error
	DeleteItem(ctx context.Context, input *DeleteItemInput) error
	BatchGetItem(ctx context.Context, input *BatchGetItemInput) (*BatchGetItemOutput, error)
	BatchWriteItem(ctx context.Context, input *BatchWriteItemInput) error
	Query(ctx context.Context, input *QueryInput) (*QueryOutput, error)
	Scan(ctx context.Context, input *ScanInput) (*ScanOutput, error)
}

// SDK-agnostic types for DynamoDB operations

type GetItemInput struct {
	TableName      string
	Key            map[string]interface{}
	ConsistentRead bool
}

type GetItemOutput struct {
	Item map[string]interface{}
}

type PutItemInput struct {
	TableName string
	Item      map[string]interface{}
}

type DeleteItemInput struct {
	TableName string
	Key       map[string]interface{}
}

type BatchGetItemInput struct {
	RequestItems map[string]*KeysAndAttributes
}

type KeysAndAttributes struct {
	Keys           []map[string]interface{}
	ConsistentRead bool
}

type BatchGetItemOutput struct {
	Responses map[string][]map[string]interface{}
}

type BatchWriteItemInput struct {
	RequestItems map[string][]*WriteRequest
}

type WriteRequest struct {
	PutRequest    *PutRequest
	DeleteRequest *DeleteRequest
}

type PutRequest struct {
	Item map[string]interface{}
}

type DeleteRequest struct {
	Key map[string]interface{}
}

type QueryInput struct {
	TableName              string
	KeyConditionExpression string
	ExpressionValues       map[string]interface{}
	ScanIndexForward       bool
	Limit                  int
}

type QueryOutput struct {
	Items []map[string]interface{}
}

type ScanInput struct {
	TableName         string
	FilterExpression  string
	ExpressionValues  map[string]interface{}
	Limit             int
	ExclusiveStartKey map[string]interface{}
}

type ScanOutput struct {
	Items            []map[string]interface{}
	LastEvaluatedKey map[string]interface{}
}

type dynamoDBStats struct {
	readOps      int64
	writeOps     int64
	bytesRead    int64
	bytesWritten int64
	errors       int64
	totalReadMs  int64
	totalWriteMs int64
}

// NewDynamoDBBackend creates a new DynamoDB backend.
func NewDynamoDBBackend(config DynamoDBConfig, client DynamoDBClient) (*DynamoDBBackend, error) {
	if config.TableName == "" {
		return nil, fmt.Errorf("%w: table name required", ErrInvalidConfig)
	}

	if config.BatchSize == 0 {
		config.BatchSize = 25
	}

	return &DynamoDBBackend{
		config:      config,
		client:      client,
		retryConfig: DefaultRetryConfig(),
	}, nil
}

// Get retrieves feature values for an entity.
func (b *DynamoDBBackend) Get(ctx context.Context, entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	start := time.Now()
	defer func() {
		atomic.AddInt64(&b.stats.totalReadMs, time.Since(start).Milliseconds())
		atomic.AddInt64(&b.stats.readOps, 1)
	}()

	input := &GetItemInput{
		TableName: b.config.TableName,
		Key: map[string]interface{}{
			"pk": entityKey,
		},
		ConsistentRead: b.config.ConsistentReads,
	}

	var output *GetItemOutput
	var err error

	err = Retry(ctx, b.retryConfig, func() error {
		output, err = b.client.GetItem(ctx, input)
		return err
	})

	if err != nil {
		atomic.AddInt64(&b.stats.errors, 1)
		return nil, err
	}

	if output == nil || output.Item == nil {
		return nil, ErrNotFound
	}

	return b.itemToFeatures(output.Item, features)
}

// Put stores feature values for an entity.
func (b *DynamoDBBackend) Put(ctx context.Context, entityKey string, features map[string]*domain.FeatureValue) error {
	if b.closed {
		return ErrBackendClosed
	}

	start := time.Now()
	defer func() {
		atomic.AddInt64(&b.stats.totalWriteMs, time.Since(start).Milliseconds())
		atomic.AddInt64(&b.stats.writeOps, 1)
	}()

	item, err := b.featuresToItem(entityKey, features)
	if err != nil {
		return err
	}

	input := &PutItemInput{
		TableName: b.config.TableName,
		Item:      item,
	}

	err = Retry(ctx, b.retryConfig, func() error {
		return b.client.PutItem(ctx, input)
	})

	if err != nil {
		atomic.AddInt64(&b.stats.errors, 1)
		return err
	}

	// Store historical version if enabled
	if b.config.HistoryEnabled && b.config.HistoryTableName != "" {
		histItem := b.featuresToHistoryItem(entityKey, features, time.Now())
		histInput := &PutItemInput{
			TableName: b.config.HistoryTableName,
			Item:      histItem,
		}
		// Best effort - don't fail main operation
		b.client.PutItem(ctx, histInput)
	}

	return nil
}

// Delete removes an entity's data.
func (b *DynamoDBBackend) Delete(ctx context.Context, entityKey string) error {
	if b.closed {
		return ErrBackendClosed
	}

	input := &DeleteItemInput{
		TableName: b.config.TableName,
		Key: map[string]interface{}{
			"pk": entityKey,
		},
	}

	return Retry(ctx, b.retryConfig, func() error {
		return b.client.DeleteItem(ctx, input)
	})
}

// GetAsOf retrieves feature values as of a specific time.
func (b *DynamoDBBackend) GetAsOf(ctx context.Context, entityKey string, features []string, asOf time.Time) (map[string]*domain.FeatureValue, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	if !b.config.HistoryEnabled || b.config.HistoryTableName == "" {
		return nil, fmt.Errorf("history not enabled")
	}

	// Query history table for the latest version before asOf
	input := &QueryInput{
		TableName:              b.config.HistoryTableName,
		KeyConditionExpression: "pk = :pk AND sk <= :ts",
		ExpressionValues: map[string]interface{}{
			":pk": entityKey,
			":ts": asOf.UnixNano(),
		},
		ScanIndexForward: false, // Descending order
		Limit:            1,
	}

	output, err := b.client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(output.Items) == 0 {
		return nil, ErrNotFound
	}

	return b.itemToFeatures(output.Items[0], features)
}

// BatchGet retrieves features for multiple entities.
func (b *DynamoDBBackend) BatchGet(ctx context.Context, entityKeys []string, features []string) (map[string]map[string]*domain.FeatureValue, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	result := make(map[string]map[string]*domain.FeatureValue)

	// Process in batches of 100 (DynamoDB limit)
	for i := 0; i < len(entityKeys); i += 100 {
		end := i + 100
		if end > len(entityKeys) {
			end = len(entityKeys)
		}
		batch := entityKeys[i:end]

		keys := make([]map[string]interface{}, len(batch))
		for j, key := range batch {
			keys[j] = map[string]interface{}{"pk": key}
		}

		input := &BatchGetItemInput{
			RequestItems: map[string]*KeysAndAttributes{
				b.config.TableName: {
					Keys:           keys,
					ConsistentRead: b.config.ConsistentReads,
				},
			},
		}

		output, err := b.client.BatchGetItem(ctx, input)
		if err != nil {
			return result, err
		}

		if items, ok := output.Responses[b.config.TableName]; ok {
			for _, item := range items {
				if pk, ok := item["pk"].(string); ok {
					featureValues, err := b.itemToFeatures(item, features)
					if err == nil {
						result[pk] = featureValues
					}
				}
			}
		}
	}

	return result, nil
}

// BatchPut stores features for multiple entities.
func (b *DynamoDBBackend) BatchPut(ctx context.Context, updates map[string]map[string]*domain.FeatureValue) error {
	if b.closed {
		return ErrBackendClosed
	}

	// Build write requests
	var requests []*WriteRequest
	for entityKey, features := range updates {
		item, err := b.featuresToItem(entityKey, features)
		if err != nil {
			return err
		}
		requests = append(requests, &WriteRequest{
			PutRequest: &PutRequest{Item: item},
		})
	}

	// Process in batches of 25 (DynamoDB limit)
	for i := 0; i < len(requests); i += 25 {
		end := i + 25
		if end > len(requests) {
			end = len(requests)
		}
		batch := requests[i:end]

		input := &BatchWriteItemInput{
			RequestItems: map[string][]*WriteRequest{
				b.config.TableName: batch,
			},
		}

		if err := b.client.BatchWriteItem(ctx, input); err != nil {
			return err
		}

		atomic.AddInt64(&b.stats.writeOps, int64(len(batch)))
	}

	return nil
}

// Scan iterates over all entities.
func (b *DynamoDBBackend) Scan(ctx context.Context, prefix string, limit int) ([]string, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	input := &ScanInput{
		TableName: b.config.TableName,
		Limit:     limit,
	}

	if prefix != "" {
		input.FilterExpression = "begins_with(pk, :prefix)"
		input.ExpressionValues = map[string]interface{}{
			":prefix": prefix,
		}
	}

	output, err := b.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, item := range output.Items {
		if pk, ok := item["pk"].(string); ok {
			keys = append(keys, pk)
		}
	}

	return keys, nil
}

// Stats returns backend statistics.
func (b *DynamoDBBackend) Stats() BackendStats {
	readOps := atomic.LoadInt64(&b.stats.readOps)
	writeOps := atomic.LoadInt64(&b.stats.writeOps)
	totalReadMs := atomic.LoadInt64(&b.stats.totalReadMs)
	totalWriteMs := atomic.LoadInt64(&b.stats.totalWriteMs)

	avgRead := float64(0)
	if readOps > 0 {
		avgRead = float64(totalReadMs) / float64(readOps)
	}

	avgWrite := float64(0)
	if writeOps > 0 {
		avgWrite = float64(totalWriteMs) / float64(writeOps)
	}

	return BackendStats{
		ReadOps:      readOps,
		WriteOps:     writeOps,
		BytesRead:    atomic.LoadInt64(&b.stats.bytesRead),
		BytesWritten: atomic.LoadInt64(&b.stats.bytesWritten),
		Errors:       atomic.LoadInt64(&b.stats.errors),
		AvgReadMs:    avgRead,
		AvgWriteMs:   avgWrite,
	}
}

// Health checks backend health.
func (b *DynamoDBBackend) Health(ctx context.Context) error {
	if b.closed {
		return ErrBackendClosed
	}

	// Try a simple scan with limit 1
	_, err := b.Scan(ctx, "", 1)
	return err
}

// Close closes the backend.
func (b *DynamoDBBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	return nil
}

// Helper methods

func (b *DynamoDBBackend) featuresToItem(entityKey string, features map[string]*domain.FeatureValue) (map[string]interface{}, error) {
	item := map[string]interface{}{
		"pk":         entityKey,
		"updated_at": time.Now().UnixNano(),
	}

	// Serialize features
	featuresJSON, err := json.Marshal(features)
	if err != nil {
		return nil, err
	}

	// Compress if enabled
	if b.config.EnableCompression {
		compressed, err := b.compress(featuresJSON)
		if err != nil {
			return nil, err
		}
		item["features"] = compressed
		item["compressed"] = true
		atomic.AddInt64(&b.stats.bytesWritten, int64(len(compressed)))
	} else {
		item["features"] = featuresJSON
		atomic.AddInt64(&b.stats.bytesWritten, int64(len(featuresJSON)))
	}

	return item, nil
}

func (b *DynamoDBBackend) featuresToHistoryItem(entityKey string, features map[string]*domain.FeatureValue, ts time.Time) map[string]interface{} {
	item := map[string]interface{}{
		"pk": entityKey,
		"sk": ts.UnixNano(),
	}

	featuresJSON, _ := json.Marshal(features)
	if b.config.EnableCompression {
		compressed, _ := b.compress(featuresJSON)
		item["features"] = compressed
		item["compressed"] = true
	} else {
		item["features"] = featuresJSON
	}

	// Set TTL for automatic expiration
	if b.config.HistoryTTL > 0 {
		item["ttl"] = time.Now().Add(b.config.HistoryTTL).Unix()
	}

	return item
}

func (b *DynamoDBBackend) itemToFeatures(item map[string]interface{}, requestedFeatures []string) (map[string]*domain.FeatureValue, error) {
	featuresData, ok := item["features"]
	if !ok {
		return nil, ErrNotFound
	}

	var featuresJSON []byte
	switch v := featuresData.(type) {
	case []byte:
		featuresJSON = v
	case string:
		featuresJSON = []byte(v)
	default:
		return nil, fmt.Errorf("invalid features data type")
	}

	// Decompress if needed
	if compressed, ok := item["compressed"].(bool); ok && compressed {
		decompressed, err := b.decompress(featuresJSON)
		if err != nil {
			return nil, err
		}
		featuresJSON = decompressed
	}

	atomic.AddInt64(&b.stats.bytesRead, int64(len(featuresJSON)))

	var allFeatures map[string]*domain.FeatureValue
	if err := json.Unmarshal(featuresJSON, &allFeatures); err != nil {
		return nil, err
	}

	// Filter to requested features
	if len(requestedFeatures) > 0 {
		result := make(map[string]*domain.FeatureValue)
		for _, name := range requestedFeatures {
			if fv, ok := allFeatures[name]; ok {
				result[name] = fv
			}
		}
		return result, nil
	}

	return allFeatures, nil
}

func (b *DynamoDBBackend) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *DynamoDBBackend) decompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

// CreateTableSchema returns the schema for creating the DynamoDB table.
func CreateTableSchema(tableName string) map[string]interface{} {
	return map[string]interface{}{
		"TableName": tableName,
		"KeySchema": []map[string]string{
			{"AttributeName": "pk", "KeyType": "HASH"},
		},
		"AttributeDefinitions": []map[string]string{
			{"AttributeName": "pk", "AttributeType": "S"},
		},
		"BillingMode": "PAY_PER_REQUEST",
	}
}

// CreateHistoryTableSchema returns the schema for the history table.
func CreateHistoryTableSchema(tableName string) map[string]interface{} {
	return map[string]interface{}{
		"TableName": tableName,
		"KeySchema": []map[string]string{
			{"AttributeName": "pk", "KeyType": "HASH"},
			{"AttributeName": "sk", "KeyType": "RANGE"},
		},
		"AttributeDefinitions": []map[string]string{
			{"AttributeName": "pk", "AttributeType": "S"},
			{"AttributeName": "sk", "AttributeType": "N"},
		},
		"BillingMode": "PAY_PER_REQUEST",
		"TimeToLiveSpecification": map[string]interface{}{
			"AttributeName": "ttl",
			"Enabled":       true,
		},
	}
}

// ParseTimestamp parses a timestamp from various formats.
func ParseTimestamp(v interface{}) (time.Time, error) {
	switch val := v.(type) {
	case int64:
		return time.Unix(0, val), nil
	case float64:
		return time.Unix(0, int64(val)), nil
	case string:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return time.Unix(0, i), nil
		}
		return time.Parse(time.RFC3339Nano, val)
	case time.Time:
		return val, nil
	default:
		return time.Time{}, fmt.Errorf("invalid timestamp type: %T", v)
	}
}

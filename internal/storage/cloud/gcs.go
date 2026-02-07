package cloud

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// GCSConfig configures the Google Cloud Storage backend.
type GCSConfig struct {
	BackendConfig

	// BucketName is the GCS bucket name.
	BucketName string `json:"bucket_name" yaml:"bucket_name"`

	// Prefix is the key prefix for all objects.
	Prefix string `json:"prefix" yaml:"prefix"`

	// CredentialsFile is the path to service account JSON.
	CredentialsFile string `json:"credentials_file,omitempty" yaml:"credentials_file,omitempty"`

	// ProjectID is the GCP project ID.
	ProjectID string `json:"project_id" yaml:"project_id"`
}

// DefaultGCSConfig returns default GCS configuration.
func DefaultGCSConfig() GCSConfig {
	return GCSConfig{
		BackendConfig: DefaultBackendConfig(),
		BucketName:    "feather-features",
		Prefix:        "features/",
	}
}

// GCSClient defines the GCS operations interface.
type GCSClient interface {
	Read(ctx context.Context, bucket, object string) ([]byte, error)
	Write(ctx context.Context, bucket, object string, data []byte) error
	Delete(ctx context.Context, bucket, object string) error
	List(ctx context.Context, bucket, prefix string, maxResults int) ([]string, error)
	Exists(ctx context.Context, bucket, object string) (bool, error)
}

// GCSBackend implements Backend using Google Cloud Storage.
type GCSBackend struct {
	config      GCSConfig
	client      GCSClient
	retryConfig RetryConfig
	stats       gcsStats
	mu          sync.RWMutex
	closed      bool
}

type gcsStats struct {
	readOps      int64
	writeOps     int64
	bytesRead    int64
	bytesWritten int64
	errors       int64
	totalReadMs  int64
	totalWriteMs int64
}

// NewGCSBackend creates a new GCS backend.
func NewGCSBackend(config GCSConfig, client GCSClient) (*GCSBackend, error) {
	if config.BucketName == "" {
		return nil, fmt.Errorf("%w: bucket name required", ErrInvalidConfig)
	}

	return &GCSBackend{
		config:      config,
		client:      client,
		retryConfig: DefaultRetryConfig(),
	}, nil
}

// objectPath returns the full object path for an entity.
func (b *GCSBackend) objectPath(entityKey string) string {
	return path.Join(b.config.Prefix, "current", entityKey+".json")
}

// historyPath returns the history object path.
func (b *GCSBackend) historyPath(entityKey string, ts time.Time) string {
	return path.Join(b.config.Prefix, "history", entityKey, fmt.Sprintf("%d.json", ts.UnixNano()))
}

// Get retrieves feature values for an entity.
func (b *GCSBackend) Get(ctx context.Context, entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	start := time.Now()
	defer func() {
		atomic.AddInt64(&b.stats.totalReadMs, time.Since(start).Milliseconds())
		atomic.AddInt64(&b.stats.readOps, 1)
	}()

	var data []byte
	var err error

	err = Retry(ctx, b.retryConfig, func() error {
		data, err = b.client.Read(ctx, b.config.BucketName, b.objectPath(entityKey))
		return err
	})

	if err != nil {
		atomic.AddInt64(&b.stats.errors, 1)
		return nil, err
	}

	if len(data) == 0 {
		return nil, ErrNotFound
	}

	return b.dataToFeatures(data, features)
}

// Put stores feature values for an entity.
func (b *GCSBackend) Put(ctx context.Context, entityKey string, features map[string]*domain.FeatureValue) error {
	if b.closed {
		return ErrBackendClosed
	}

	start := time.Now()
	defer func() {
		atomic.AddInt64(&b.stats.totalWriteMs, time.Since(start).Milliseconds())
		atomic.AddInt64(&b.stats.writeOps, 1)
	}()

	data, err := b.featuresToData(features)
	if err != nil {
		return err
	}

	err = Retry(ctx, b.retryConfig, func() error {
		return b.client.Write(ctx, b.config.BucketName, b.objectPath(entityKey), data)
	})

	if err != nil {
		atomic.AddInt64(&b.stats.errors, 1)
		return err
	}

	atomic.AddInt64(&b.stats.bytesWritten, int64(len(data)))

	// Write history if enabled
	if b.config.HistoryEnabled {
		histPath := b.historyPath(entityKey, time.Now())
		if err := b.client.Write(ctx, b.config.BucketName, histPath, data); err != nil {
			atomic.AddInt64(&b.stats.errors, 1)
		}
	}

	return nil
}

// Delete removes an entity's data.
func (b *GCSBackend) Delete(ctx context.Context, entityKey string) error {
	if b.closed {
		return ErrBackendClosed
	}

	return Retry(ctx, b.retryConfig, func() error {
		return b.client.Delete(ctx, b.config.BucketName, b.objectPath(entityKey))
	})
}

// GetAsOf retrieves feature values as of a specific time.
func (b *GCSBackend) GetAsOf(ctx context.Context, entityKey string, features []string, asOf time.Time) (map[string]*domain.FeatureValue, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	if !b.config.HistoryEnabled {
		return nil, fmt.Errorf("history not enabled")
	}

	// List history objects and find the latest before asOf
	prefix := path.Join(b.config.Prefix, "history", entityKey) + "/"
	objects, err := b.client.List(ctx, b.config.BucketName, prefix, 1000)
	if err != nil {
		return nil, err
	}

	var latestPath string
	latestTs := int64(0)
	targetTs := asOf.UnixNano()

	for _, obj := range objects {
		// Extract timestamp from filename
		var ts int64
		if _, scanErr := fmt.Sscanf(path.Base(obj), "%d.json", &ts); scanErr != nil {
			continue
		}

		if ts <= targetTs && ts > latestTs {
			latestTs = ts
			latestPath = obj
		}
	}

	if latestPath == "" {
		return nil, ErrNotFound
	}

	data, err := b.client.Read(ctx, b.config.BucketName, latestPath)
	if err != nil {
		return nil, err
	}

	return b.dataToFeatures(data, features)
}

// BatchGet retrieves features for multiple entities.
func (b *GCSBackend) BatchGet(ctx context.Context, entityKeys []string, features []string) (map[string]map[string]*domain.FeatureValue, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	result := make(map[string]map[string]*domain.FeatureValue)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(entityKeys))

	// Concurrent reads with limited parallelism
	sem := make(chan struct{}, 10)

	for _, key := range entityKeys {
		wg.Add(1)
		go func(entityKey string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			featureValues, err := b.Get(ctx, entityKey, features)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return
				}
				errChan <- err
				return
			}

			if featureValues != nil {
				mu.Lock()
				result[entityKey] = featureValues
				mu.Unlock()
			}
		}(key)
	}

	wg.Wait()
	close(errChan)

	// Return first error if any
	for err := range errChan {
		return result, err
	}

	return result, nil
}

// BatchPut stores features for multiple entities.
func (b *GCSBackend) BatchPut(ctx context.Context, updates map[string]map[string]*domain.FeatureValue) error {
	if b.closed {
		return ErrBackendClosed
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(updates))
	sem := make(chan struct{}, 10)

	for entityKey, features := range updates {
		wg.Add(1)
		go func(key string, feats map[string]*domain.FeatureValue) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := b.Put(ctx, key, feats); err != nil {
				errChan <- err
			}
		}(entityKey, features)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		return err
	}

	return nil
}

// Scan iterates over all entities.
func (b *GCSBackend) Scan(ctx context.Context, prefix string, limit int) ([]string, error) {
	if b.closed {
		return nil, ErrBackendClosed
	}

	fullPrefix := path.Join(b.config.Prefix, "current")
	if prefix != "" {
		fullPrefix = path.Join(fullPrefix, prefix)
	}

	objects, err := b.client.List(ctx, b.config.BucketName, fullPrefix, limit)
	if err != nil {
		return nil, err
	}

	// Extract entity keys from object paths
	var keys []string
	for _, obj := range objects {
		base := path.Base(obj)
		if len(base) > 5 { // Remove .json suffix
			keys = append(keys, base[:len(base)-5])
		}
	}

	return keys, nil
}

// Stats returns backend statistics.
func (b *GCSBackend) Stats() BackendStats {
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
func (b *GCSBackend) Health(ctx context.Context) error {
	if b.closed {
		return ErrBackendClosed
	}

	// Try listing a few objects
	_, err := b.client.List(ctx, b.config.BucketName, b.config.Prefix, 1)
	return err
}

// Close closes the backend.
func (b *GCSBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	return nil
}

// Helper methods

func (b *GCSBackend) featuresToData(features map[string]*domain.FeatureValue) ([]byte, error) {
	data, err := json.Marshal(features)
	if err != nil {
		return nil, err
	}

	if b.config.EnableCompression {
		return b.compress(data)
	}

	return data, nil
}

func (b *GCSBackend) dataToFeatures(data []byte, requestedFeatures []string) (map[string]*domain.FeatureValue, error) {
	// Check if compressed (gzip magic number)
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		decompressed, err := b.decompress(data)
		if err != nil {
			return nil, err
		}
		data = decompressed
	}

	atomic.AddInt64(&b.stats.bytesRead, int64(len(data)))

	var allFeatures map[string]*domain.FeatureValue
	if err := json.Unmarshal(data, &allFeatures); err != nil {
		return nil, err
	}

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

func (b *GCSBackend) compress(data []byte) ([]byte, error) {
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

func (b *GCSBackend) decompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = gz.Close()
	}()
	return io.ReadAll(gz)
}

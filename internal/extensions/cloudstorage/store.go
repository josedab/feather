package cloudstorage

import (
	"crypto/md5"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Provider identifies a cloud storage provider.
type Provider string

const (
	ProviderLocal Provider = "local"
	ProviderS3    Provider = "s3"
	ProviderGCS   Provider = "gcs"
	ProviderAzure Provider = "azure"
	ProviderMinIO Provider = "minio"
)

// ObjectInfo contains metadata about a stored object.
type ObjectInfo struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type"`
	LastModified time.Time         `json:"last_modified"`
	ETag         string            `json:"etag"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// StoreConfig configures the object store.
type StoreConfig struct {
	Provider   Provider `json:"provider"`
	Bucket     string   `json:"bucket"`
	Prefix     string   `json:"prefix"`
	Region     string   `json:"region"`
	Endpoint   string   `json:"endpoint"`
	MaxObjects int      `json:"max_objects"`
}

// DefaultStoreConfig returns sensible defaults.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		Provider:   ProviderLocal,
		Bucket:     "feather-data",
		Prefix:     "",
		MaxObjects: 10000000,
	}
}

// BucketStats holds bucket-level statistics.
type BucketStats struct {
	TotalObjects  int    `json:"total_objects"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
	Provider      string `json:"provider"`
	Bucket        string `json:"bucket"`
}

// StoreStats holds store-level statistics.
type StoreStats struct {
	TotalPuts      int64  `json:"total_puts"`
	TotalGets      int64  `json:"total_gets"`
	TotalDeletes   int64  `json:"total_deletes"`
	TotalObjects   int    `json:"total_objects"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
	Provider       string `json:"provider"`
}

type storedObject struct {
	info ObjectInfo
	data []byte
}

// ObjectStore provides a pluggable object storage abstraction.
type ObjectStore struct {
	mu           sync.RWMutex
	config       StoreConfig
	objects      map[string]*storedObject
	totalPuts    atomic.Int64
	totalGets    atomic.Int64
	totalDeletes atomic.Int64
}

// NewObjectStore creates a new object store.
func NewObjectStore(config StoreConfig) *ObjectStore {
	return &ObjectStore{
		config:  config,
		objects: make(map[string]*storedObject),
	}
}

// Put stores an object with the given key.
func (s *ObjectStore) Put(key string, data []byte, contentType string, metadata map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.objects) >= s.config.MaxObjects {
		if _, exists := s.objects[key]; !exists {
			return fmt.Errorf("max objects (%d) reached", s.config.MaxObjects)
		}
	}

	hash := md5.Sum(data)
	etag := fmt.Sprintf("%x", hash)

	s.objects[key] = &storedObject{
		info: ObjectInfo{
			Key:          key,
			Size:         int64(len(data)),
			ContentType:  contentType,
			LastModified: time.Now(),
			ETag:         etag,
			Metadata:     metadata,
		},
		data: append([]byte(nil), data...),
	}
	s.totalPuts.Add(1)
	return nil
}

// Get retrieves an object by key.
func (s *ObjectStore) Get(key string) ([]byte, *ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, exists := s.objects[key]
	if !exists {
		return nil, nil, fmt.Errorf("key %s: %w", key, ErrObjectNotFound)
	}

	data := append([]byte(nil), obj.data...)
	info := obj.info
	s.totalGets.Add(1)
	return data, &info, nil
}

// Delete removes an object by key.
func (s *ObjectStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.objects[key]; !exists {
		return fmt.Errorf("key %s: %w", key, ErrObjectNotFound)
	}

	delete(s.objects, key)
	s.totalDeletes.Add(1)
	return nil
}

// List returns objects matching the given prefix, up to limit.
func (s *ObjectStore) List(prefix string, limit int) []ObjectInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []ObjectInfo
	for _, obj := range s.objects {
		if prefix == "" || strings.HasPrefix(obj.info.Key, prefix) {
			results = append(results, obj.info)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}

// Exists checks whether an object exists.
func (s *ObjectStore) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.objects[key]
	return exists
}

// GetInfo returns metadata for an object without retrieving its data.
func (s *ObjectStore) GetInfo(key string) (*ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, exists := s.objects[key]
	if !exists {
		return nil, fmt.Errorf("key %s: %w", key, ErrObjectNotFound)
	}

	info := obj.info
	return &info, nil
}

// Copy duplicates an object from srcKey to dstKey.
func (s *ObjectStore) Copy(srcKey, dstKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, exists := s.objects[srcKey]
	if !exists {
		return fmt.Errorf("source key %s: %w", srcKey, ErrObjectNotFound)
	}

	data := append([]byte(nil), src.data...)
	meta := make(map[string]string, len(src.info.Metadata))
	for k, v := range src.info.Metadata {
		meta[k] = v
	}

	hash := md5.Sum(data)
	s.objects[dstKey] = &storedObject{
		info: ObjectInfo{
			Key:          dstKey,
			Size:         int64(len(data)),
			ContentType:  src.info.ContentType,
			LastModified: time.Now(),
			ETag:         fmt.Sprintf("%x", hash),
			Metadata:     meta,
		},
		data: data,
	}
	s.totalPuts.Add(1)
	return nil
}

// ListBucketStats returns aggregate statistics for the bucket.
func (s *ObjectStore) ListBucketStats() BucketStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalSize int64
	for _, obj := range s.objects {
		totalSize += obj.info.Size
	}

	return BucketStats{
		TotalObjects:   len(s.objects),
		TotalSizeBytes: totalSize,
		Provider:       string(s.config.Provider),
		Bucket:         s.config.Bucket,
	}
}

// Stats returns store-level statistics.
func (s *ObjectStore) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalSize int64
	for _, obj := range s.objects {
		totalSize += obj.info.Size
	}

	return StoreStats{
		TotalPuts:      s.totalPuts.Load(),
		TotalGets:      s.totalGets.Load(),
		TotalDeletes:   s.totalDeletes.Load(),
		TotalObjects:   len(s.objects),
		TotalSizeBytes: totalSize,
		Provider:       string(s.config.Provider),
	}
}

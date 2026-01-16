package multimodal

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ModalityType identifies the type of multi-modal content.
type ModalityType string

const (
	ModalityImage     ModalityType = "image"
	ModalityVideo     ModalityType = "video"
	ModalityAudio     ModalityType = "audio"
	ModalityText      ModalityType = "text"
	ModalityEmbedding ModalityType = "embedding"
	ModalityDocument  ModalityType = "document"
)

// CompressionType identifies the compression algorithm.
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
	CompressionLZ4  CompressionType = "lz4"
	CompressionZstd CompressionType = "zstd"
)

var (
	ErrBlobNotFound    = errors.New("blob not found")
	ErrBlobTooLarge    = errors.New("blob exceeds maximum size")
	ErrDuplicateBlob   = errors.New("duplicate blob")
	ErrInvalidModality = errors.New("invalid modality type")
	ErrHashNotFound    = errors.New("blob hash not found")
)

// StoreConfig configures the MultiModalStore.
type StoreConfig struct {
	MaxBlobSize          int64
	DefaultCompression   CompressionType
	EnableDeduplication  bool
	EnableLazyLoading    bool
	MaxCacheSize         int
	ThumbnailSize        int
}

// DefaultStoreConfig returns sensible defaults for the multi-modal store.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxBlobSize:         100 * 1024 * 1024, // 100MB
		DefaultCompression:  CompressionGzip,
		EnableDeduplication: true,
		EnableLazyLoading:   true,
		MaxCacheSize:        1000,
		ThumbnailSize:       256,
	}
}

// Dimensions describes spatial dimensions of image/video content.
type Dimensions struct {
	Width    int `json:"width"`
	Height   int `json:"height"`
	Channels int `json:"channels"`
}

// BlobMetadata stores metadata for a multi-modal blob.
type BlobMetadata struct {
	ID             string            `json:"id"`
	Modality       ModalityType      `json:"modality"`
	ContentType    string            `json:"content_type"`
	OriginalSize   int64             `json:"original_size"`
	CompressedSize int64             `json:"compressed_size"`
	Compression    CompressionType   `json:"compression"`
	Hash           string            `json:"hash"`
	Dimensions     *Dimensions       `json:"dimensions,omitempty"`
	Duration       *time.Duration    `json:"duration,omitempty"`
	SampleRate     int               `json:"sample_rate,omitempty"`
	Embedding      []float64         `json:"embedding,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	AccessCount    int64             `json:"access_count"`
	LastAccess     time.Time         `json:"last_access"`
}

// StoreStats holds aggregate statistics for the multi-modal store.
type StoreStats struct {
	TotalBlobs        int            `json:"total_blobs"`
	TotalSize         int64          `json:"total_size"`
	CompressedSize    int64          `json:"compressed_size"`
	CompressionRatio  float64        `json:"compression_ratio"`
	DuplicatesAvoided int           `json:"duplicates_avoided"`
	BlobsByModality   map[string]int `json:"blobs_by_modality"`
	AvgBlobSize       int64          `json:"avg_blob_size"`
	CacheHitRate      float64        `json:"cache_hit_rate"`
}

// MultiModalStore manages multi-modal blob storage with compression and dedup.
type MultiModalStore struct {
	mu       sync.RWMutex
	config   StoreConfig
	blobs    map[string][]byte       // id -> compressed data
	metadata map[string]*BlobMetadata // id -> metadata
	hashIdx  map[string]string       // hash -> id (for dedup)
	nextID   int

	duplicatesAvoided atomic.Int64
	cacheHits         atomic.Int64
	cacheMisses       atomic.Int64
}

// NewMultiModalStore creates a new multi-modal store.
func NewMultiModalStore(config StoreConfig) *MultiModalStore {
	return &MultiModalStore{
		config:   config,
		blobs:    make(map[string][]byte),
		metadata: make(map[string]*BlobMetadata),
		hashIdx:  make(map[string]string),
	}
}

// Store stores a blob with compression and deduplication.
func (s *MultiModalStore) Store(modality ModalityType, contentType string, data []byte, tags map[string]string) (*BlobMetadata, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty data", ErrInvalidModality)
	}
	if int64(len(data)) > s.config.MaxBlobSize {
		return nil, fmt.Errorf("%w: size %d exceeds limit %d", ErrBlobTooLarge, len(data), s.config.MaxBlobSize)
	}

	hash := computeHash(data)

	// Deduplication check
	if s.config.EnableDeduplication {
		s.mu.RLock()
		if existingID, ok := s.hashIdx[hash]; ok {
			meta := s.metadata[existingID]
			s.mu.RUnlock()
			s.duplicatesAvoided.Add(1)
			return meta, nil
		}
		s.mu.RUnlock()
	}

	compressed, err := compress(data, s.config.DefaultCompression)
	if err != nil {
		return nil, fmt.Errorf("compressing blob: %w", err)
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	id := fmt.Sprintf("blob_%d", s.nextID)

	meta := &BlobMetadata{
		ID:             id,
		Modality:       modality,
		ContentType:    contentType,
		OriginalSize:   int64(len(data)),
		CompressedSize: int64(len(compressed)),
		Compression:    s.config.DefaultCompression,
		Hash:           hash,
		Tags:           tags,
		CreatedAt:      now,
		LastAccess:     now,
	}

	s.blobs[id] = compressed
	s.metadata[id] = meta
	s.hashIdx[hash] = id

	return meta, nil
}

// Get retrieves a blob, decompressing on read.
func (s *MultiModalStore) Get(id string) ([]byte, *BlobMetadata, error) {
	s.mu.RLock()
	compressed, ok := s.blobs[id]
	meta := s.metadata[id]
	s.mu.RUnlock()

	if !ok {
		s.cacheMisses.Add(1)
		return nil, nil, ErrBlobNotFound
	}

	s.cacheHits.Add(1)

	data, err := decompress(compressed, meta.Compression)
	if err != nil {
		return nil, nil, fmt.Errorf("decompressing blob: %w", err)
	}

	// Update access stats
	s.mu.Lock()
	meta.AccessCount++
	meta.LastAccess = time.Now()
	s.mu.Unlock()

	return data, meta, nil
}

// GetMetadata retrieves only metadata (lazy loading).
func (s *MultiModalStore) GetMetadata(id string) (*BlobMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.metadata[id]
	if !ok {
		return nil, ErrBlobNotFound
	}
	return meta, nil
}

// Delete removes a blob from the store.
func (s *MultiModalStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.metadata[id]
	if !ok {
		return ErrBlobNotFound
	}

	delete(s.hashIdx, meta.Hash)
	delete(s.blobs, id)
	delete(s.metadata, id)
	return nil
}

// List returns all blobs matching the given modality. Pass empty string for all.
func (s *MultiModalStore) List(modality ModalityType) []*BlobMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*BlobMetadata, 0)
	for _, meta := range s.metadata {
		if modality == "" || meta.Modality == modality {
			result = append(result, meta)
		}
	}
	return result
}

// Search searches blobs by matching query against tags.
func (s *MultiModalStore) Search(query string) []*BlobMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	result := make([]*BlobMetadata, 0)
	for _, meta := range s.metadata {
		if matchesTags(meta, q) {
			result = append(result, meta)
		}
	}
	return result
}

// GetByHash performs a content-addressed lookup by SHA-256 hash.
func (s *MultiModalStore) GetByHash(hash string) (*BlobMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.hashIdx[hash]
	if !ok {
		return nil, ErrHashNotFound
	}
	return s.metadata[id], nil
}

// Stats returns aggregate store statistics.
func (s *MultiModalStore) Stats() *StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StoreStats{
		TotalBlobs:        len(s.metadata),
		BlobsByModality:   make(map[string]int),
		DuplicatesAvoided: int(s.duplicatesAvoided.Load()),
	}

	for _, meta := range s.metadata {
		stats.TotalSize += meta.OriginalSize
		stats.CompressedSize += meta.CompressedSize
		stats.BlobsByModality[string(meta.Modality)]++
	}

	if stats.TotalBlobs > 0 {
		stats.AvgBlobSize = stats.TotalSize / int64(stats.TotalBlobs)
	}
	if stats.TotalSize > 0 {
		stats.CompressionRatio = float64(stats.CompressedSize) / float64(stats.TotalSize)
	}

	hits := s.cacheHits.Load()
	misses := s.cacheMisses.Load()
	total := hits + misses
	if total > 0 {
		stats.CacheHitRate = float64(hits) / float64(total)
	}

	return stats
}

func matchesTags(meta *BlobMetadata, query string) bool {
	for k, v := range meta.Tags {
		if strings.Contains(strings.ToLower(k), query) || strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

func computeHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func compress(data []byte, compression CompressionType) ([]byte, error) {
	switch compression {
	case CompressionNone:
		return data, nil
	case CompressionGzip:
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case CompressionLZ4, CompressionZstd:
		// Fallback to no compression for unsupported types
		return data, nil
	default:
		return data, nil
	}
}

func decompress(data []byte, compression CompressionType) ([]byte, error) {
	switch compression {
	case CompressionNone:
		return data, nil
	case CompressionGzip:
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	case CompressionLZ4, CompressionZstd:
		return data, nil
	default:
		return data, nil
	}
}

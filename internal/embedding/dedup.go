package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"sync/atomic"
)

// DeduplicationConfig configures embedding deduplication.
type DeduplicationConfig struct {
	// Enabled enables deduplication.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// NormalizeContent normalizes content before hashing.
	NormalizeContent bool `json:"normalize_content" yaml:"normalize_content"`

	// IncludeModelInHash includes model ID in the hash.
	IncludeModelInHash bool `json:"include_model_in_hash" yaml:"include_model_in_hash"`

	// HashAlgorithm is the hashing algorithm (sha256, xxhash).
	HashAlgorithm string `json:"hash_algorithm" yaml:"hash_algorithm"`
}

// DefaultDeduplicationConfig returns the default deduplication configuration.
func DefaultDeduplicationConfig() DeduplicationConfig {
	return DeduplicationConfig{
		Enabled:            true,
		NormalizeContent:   true,
		IncludeModelInHash: true,
		HashAlgorithm:      "sha256",
	}
}

// Deduplicator handles embedding deduplication.
type Deduplicator struct {
	mu     sync.RWMutex
	config DeduplicationConfig
	store  *Store

	// Metrics
	duplicatesFound int64
	uniqueContent   int64
	bytesDeduped    int64
}

// NewDeduplicator creates a new deduplicator.
func NewDeduplicator(config DeduplicationConfig, store *Store) *Deduplicator {
	return &Deduplicator{
		config: config,
		store:  store,
	}
}

// HashContent generates a content hash for deduplication.
func (d *Deduplicator) HashContent(content string, modelID string) string {
	if !d.config.Enabled {
		return ""
	}

	input := content
	if d.config.NormalizeContent {
		input = normalizeContent(content)
	}

	if d.config.IncludeModelInHash {
		input = modelID + ":" + input
	}

	switch d.config.HashAlgorithm {
	case "sha256":
		hash := sha256.Sum256([]byte(input))
		return hex.EncodeToString(hash[:])
	default:
		hash := sha256.Sum256([]byte(input))
		return hex.EncodeToString(hash[:])
	}
}

// CheckDuplicate checks if content already has an embedding.
func (d *Deduplicator) CheckDuplicate(ctx context.Context, content string, modelID string) (*Embedding, bool) {
	if !d.config.Enabled {
		return nil, false
	}

	hash := d.HashContent(content, modelID)
	if hash == "" {
		return nil, false
	}

	emb, err := d.store.GetByHash(ctx, hash)
	if err != nil {
		return nil, false
	}

	atomic.AddInt64(&d.duplicatesFound, 1)
	return emb, true
}

// GetOrCreate returns existing embedding or signals to create new.
func (d *Deduplicator) GetOrCreate(ctx context.Context, content string, modelID string) (*Embedding, bool, error) {
	if !d.config.Enabled {
		return nil, false, nil
	}

	// Check for existing embedding
	hash := d.HashContent(content, modelID)
	if hash == "" {
		return nil, false, nil
	}

	emb, err := d.store.GetByHash(ctx, hash)
	if err == nil {
		atomic.AddInt64(&d.duplicatesFound, 1)
		return emb, true, nil
	}

	if err != ErrEmbeddingNotFound {
		return nil, false, err
	}

	atomic.AddInt64(&d.uniqueContent, 1)
	return nil, false, nil
}

// RecordDeduplication records deduplication savings.
func (d *Deduplicator) RecordDeduplication(bytesSaved int64) {
	atomic.AddInt64(&d.bytesDeduped, bytesSaved)
}

// Stats returns deduplication statistics.
func (d *Deduplicator) Stats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":          d.config.Enabled,
		"duplicates_found": atomic.LoadInt64(&d.duplicatesFound),
		"unique_content":   atomic.LoadInt64(&d.uniqueContent),
		"bytes_deduped":    atomic.LoadInt64(&d.bytesDeduped),
		"hash_algorithm":   d.config.HashAlgorithm,
	}
}

// normalizeContent normalizes content for consistent hashing.
func normalizeContent(content string) string {
	// Basic normalization: trim whitespace, lowercase
	// Could be extended for more sophisticated normalization
	result := make([]byte, 0, len(content))
	lastWasSpace := true

	for i := 0; i < len(content); i++ {
		c := content[i]

		// Convert to lowercase
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}

		// Collapse whitespace
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if !lastWasSpace {
				result = append(result, ' ')
				lastWasSpace = true
			}
			continue
		}

		result = append(result, c)
		lastWasSpace = false
	}

	// Trim trailing space
	if len(result) > 0 && result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}

	return string(result)
}

// ContentHasher provides standalone hashing functionality.
type ContentHasher struct {
	algorithm string
}

// NewContentHasher creates a new content hasher.
func NewContentHasher(algorithm string) *ContentHasher {
	if algorithm == "" {
		algorithm = "sha256"
	}
	return &ContentHasher{algorithm: algorithm}
}

// Hash generates a hash for the given content.
func (h *ContentHasher) Hash(content string) string {
	switch h.algorithm {
	case "sha256":
		hash := sha256.Sum256([]byte(content))
		return hex.EncodeToString(hash[:])
	default:
		hash := sha256.Sum256([]byte(content))
		return hex.EncodeToString(hash[:])
	}
}

// HashWithModel generates a hash including model information.
func (h *ContentHasher) HashWithModel(content, modelID, modelVersion string) string {
	input := modelID + ":" + modelVersion + ":" + content
	return h.Hash(input)
}

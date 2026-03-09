package cloudstorage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ColdTierConfig configures the cold storage tier.
type ColdTierConfig struct {
	Backend         Backend       `json:"-"`
	ArchiveAfter    time.Duration `json:"archive_after" yaml:"archive_after"`
	PromotionTTL    time.Duration `json:"promotion_ttl" yaml:"promotion_ttl"`
	ArchiveInterval time.Duration `json:"archive_interval" yaml:"archive_interval"`
	MaxArchiveSize  int64         `json:"max_archive_size" yaml:"max_archive_size"`
}

// DefaultColdTierConfig returns sensible defaults.
func DefaultColdTierConfig() ColdTierConfig {
	return ColdTierConfig{
		ArchiveAfter:    24 * time.Hour,
		PromotionTTL:    1 * time.Hour,
		ArchiveInterval: 5 * time.Minute,
		MaxArchiveSize:  100 * 1024 * 1024 * 1024, // 100GB
	}
}

// ArchiveRecord tracks metadata for archived objects.
type ArchiveRecord struct {
	Key         string    `json:"key"`
	Size        int64     `json:"size"`
	ArchivedAt  time.Time `json:"archived_at"`
	OriginalKey string    `json:"original_key"`
	ContentType string    `json:"content_type"`
}

// ColdTierStats tracks cold tier statistics.
type ColdTierStats struct {
	TotalArchived   int64 `json:"total_archived"`
	TotalPromoted   int64 `json:"total_promoted"`
	TotalSizeBytes  int64 `json:"total_size_bytes"`
	ArchiveErrors   int64 `json:"archive_errors"`
	PromotionErrors int64 `json:"promotion_errors"`
}

// ColdTier provides archival storage with background archival and async promotion.
type ColdTier struct {
	mu       sync.RWMutex
	config   ColdTierConfig
	backend  Backend
	archives map[string]*ArchiveRecord
	stats    ColdTierStats
	logger   *slog.Logger
	stopCh   chan struct{}
}

// NewColdTier creates a new cold storage tier.
func NewColdTier(config ColdTierConfig) (*ColdTier, error) {
	if config.Backend == nil {
		return nil, fmt.Errorf("cold tier backend is required")
	}
	return &ColdTier{
		config:   config,
		backend:  config.Backend,
		archives: make(map[string]*ArchiveRecord),
		logger:   slog.Default(),
		stopCh:   make(chan struct{}),
	}, nil
}

// Archive stores data in cold storage.
func (c *ColdTier) Archive(ctx context.Context, key string, data []byte, contentType string) error {
	archiveKey := "archive/" + key
	if err := c.backend.Put(ctx, archiveKey, data, contentType, map[string]string{
		"archived_at":  time.Now().Format(time.RFC3339),
		"original_key": key,
	}); err != nil {
		c.mu.Lock()
		c.stats.ArchiveErrors++
		c.mu.Unlock()
		return fmt.Errorf("archiving %s: %w", key, err)
	}

	c.mu.Lock()
	c.archives[key] = &ArchiveRecord{
		Key:         archiveKey,
		Size:        int64(len(data)),
		ArchivedAt:  time.Now(),
		OriginalKey: key,
		ContentType: contentType,
	}
	c.stats.TotalArchived++
	c.stats.TotalSizeBytes += int64(len(data))
	c.mu.Unlock()

	return nil
}

// Promote retrieves data from cold storage for online serving.
func (c *ColdTier) Promote(ctx context.Context, key string) ([]byte, *ObjectInfo, error) {
	archiveKey := "archive/" + key
	data, info, err := c.backend.Get(ctx, archiveKey)
	if err != nil {
		c.mu.Lock()
		c.stats.PromotionErrors++
		c.mu.Unlock()
		return nil, nil, fmt.Errorf("promoting %s from cold tier: %w", key, err)
	}

	c.mu.Lock()
	c.stats.TotalPromoted++
	c.mu.Unlock()

	return data, info, nil
}

// IsArchived checks if a key exists in cold storage.
func (c *ColdTier) IsArchived(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.archives[key]
	return exists
}

// ListArchives returns all archived records.
func (c *ColdTier) ListArchives() []ArchiveRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]ArchiveRecord, 0, len(c.archives))
	for _, r := range c.archives {
		result = append(result, *r)
	}
	return result
}

// Delete removes an object from cold storage.
func (c *ColdTier) Delete(ctx context.Context, key string) error {
	archiveKey := "archive/" + key
	if err := c.backend.Delete(ctx, archiveKey); err != nil {
		return fmt.Errorf("deleting %s from cold tier: %w", key, err)
	}
	c.mu.Lock()
	if record, exists := c.archives[key]; exists {
		c.stats.TotalSizeBytes -= record.Size
	}
	delete(c.archives, key)
	c.mu.Unlock()
	return nil
}

// Stats returns cold tier statistics.
func (c *ColdTier) Stats() ColdTierStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Close stops the cold tier.
func (c *ColdTier) Close() error {
	close(c.stopCh)
	return c.backend.Close()
}

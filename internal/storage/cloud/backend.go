package cloud

import (
	"context"
	"errors"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// Common errors.
var (
	ErrNotFound         = errors.New("item not found")
	ErrBackendClosed    = errors.New("backend is closed")
	ErrInvalidConfig    = errors.New("invalid configuration")
	ErrConnectionFailed = errors.New("connection failed")
	ErrTimeout          = errors.New("operation timed out")
	ErrQuotaExceeded    = errors.New("quota exceeded")
)

// Backend defines the interface for cloud storage backends.
type Backend interface {
	// Get retrieves feature values for an entity.
	Get(ctx context.Context, entityKey string, features []string) (map[string]*domain.FeatureValue, error)

	// Put stores feature values for an entity.
	Put(ctx context.Context, entityKey string, features map[string]*domain.FeatureValue) error

	// Delete removes an entity's data.
	Delete(ctx context.Context, entityKey string) error

	// GetAsOf retrieves feature values as of a specific time.
	GetAsOf(ctx context.Context, entityKey string, features []string, asOf time.Time) (map[string]*domain.FeatureValue, error)

	// BatchGet retrieves features for multiple entities.
	BatchGet(ctx context.Context, entityKeys []string, features []string) (map[string]map[string]*domain.FeatureValue, error)

	// BatchPut stores features for multiple entities.
	BatchPut(ctx context.Context, updates map[string]map[string]*domain.FeatureValue) error

	// Scan iterates over all entities.
	Scan(ctx context.Context, prefix string, limit int) ([]string, error)

	// Stats returns backend statistics.
	Stats() BackendStats

	// Health checks backend health.
	Health(ctx context.Context) error

	// Close closes the backend.
	Close() error
}

// BackendStats contains backend statistics.
type BackendStats struct {
	ReadOps           int64   `json:"read_ops"`
	WriteOps          int64   `json:"write_ops"`
	BytesRead         int64   `json:"bytes_read"`
	BytesWritten      int64   `json:"bytes_written"`
	Errors            int64   `json:"errors"`
	AvgReadMs         float64 `json:"avg_read_ms"`
	AvgWriteMs        float64 `json:"avg_write_ms"`
	ItemCount         int64   `json:"item_count"`
	StorageBytes      int64   `json:"storage_bytes"`
	ConnectionsActive int     `json:"connections_active"`
}

// BackendConfig contains common configuration options.
type BackendConfig struct {
	// Region is the cloud region.
	Region string `json:"region" yaml:"region"`

	// Endpoint is an optional custom endpoint (for local testing).
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`

	// Timeout is the default operation timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// MaxRetries is the maximum number of retries.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// RetryDelay is the base delay between retries.
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`

	// EnableCompression enables data compression.
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression"`

	// HistoryEnabled enables historical versioning.
	HistoryEnabled bool `json:"history_enabled" yaml:"history_enabled"`

	// HistoryTTL is how long to keep historical versions.
	HistoryTTL time.Duration `json:"history_ttl" yaml:"history_ttl"`
}

// DefaultBackendConfig returns default configuration.
func DefaultBackendConfig() BackendConfig {
	return BackendConfig{
		Region:            "us-east-1",
		Timeout:           30 * time.Second,
		MaxRetries:        3,
		RetryDelay:        100 * time.Millisecond,
		EnableCompression: true,
		HistoryEnabled:    true,
		HistoryTTL:        30 * 24 * time.Hour, // 30 days
	}
}

// RetryConfig for retry behavior.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// DefaultRetryConfig returns default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Multiplier: 2.0,
	}
}

// Retry executes fn with exponential backoff.
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	delay := config.BaseDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == config.MaxRetries {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return lastErr
}

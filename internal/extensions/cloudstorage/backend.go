package cloudstorage

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Backend is the interface for cloud storage provider implementations.
type Backend interface {
	Put(ctx context.Context, key string, data []byte, contentType string, metadata map[string]string) error
	Get(ctx context.Context, key string) ([]byte, *ObjectInfo, error)
	GetRange(ctx context.Context, key string, offset, length int64) ([]byte, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string, limit int) ([]ObjectInfo, error)
	Copy(ctx context.Context, srcKey, dstKey string) error
	Exists(ctx context.Context, key string) (bool, error)
	Close() error
}

// RetryConfig configures retry behavior for storage operations.
type RetryConfig struct {
	MaxRetries     int           `json:"max_retries" yaml:"max_retries"`
	InitialBackoff time.Duration `json:"initial_backoff" yaml:"initial_backoff"`
	MaxBackoff     time.Duration `json:"max_backoff" yaml:"max_backoff"`
	BackoffFactor  float64       `json:"backoff_factor" yaml:"backoff_factor"`
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
	}
}

// RetryableBackend wraps a Backend with retry logic and exponential backoff.
type RetryableBackend struct {
	backend Backend
	config  RetryConfig
}

// NewRetryableBackend wraps a backend with retry logic.
func NewRetryableBackend(backend Backend, config RetryConfig) *RetryableBackend {
	if config.MaxRetries == 0 {
		config = DefaultRetryConfig()
	}
	return &RetryableBackend{backend: backend, config: config}
}

func (r *RetryableBackend) retry(ctx context.Context, op string, fn func() error) error {
	var lastErr error
	backoff := r.config.InitialBackoff
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if attempt < r.config.MaxRetries {
				// Add jitter
				halfBackoff := int64(backoff) / 2
				var jitter time.Duration
				if halfBackoff > 0 {
					jitter = time.Duration(rand.Int63n(halfBackoff))
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff + jitter):
				}
				backoff = time.Duration(float64(backoff) * r.config.BackoffFactor)
				if backoff > r.config.MaxBackoff {
					backoff = r.config.MaxBackoff
				}
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("%s failed after %d retries: %w", op, r.config.MaxRetries, lastErr)
}

// Put stores an object with retry logic.
func (r *RetryableBackend) Put(ctx context.Context, key string, data []byte, contentType string, metadata map[string]string) error {
	return r.retry(ctx, "put", func() error {
		return r.backend.Put(ctx, key, data, contentType, metadata)
	})
}

// Get retrieves an object with retry logic.
func (r *RetryableBackend) Get(ctx context.Context, key string) ([]byte, *ObjectInfo, error) {
	var data []byte
	var info *ObjectInfo
	err := r.retry(ctx, "get", func() error {
		var e error
		data, info, e = r.backend.Get(ctx, key)
		return e
	})
	return data, info, err
}

// GetRange retrieves a range of bytes with retry logic.
func (r *RetryableBackend) GetRange(ctx context.Context, key string, offset, length int64) ([]byte, error) {
	var data []byte
	err := r.retry(ctx, "get_range", func() error {
		var e error
		data, e = r.backend.GetRange(ctx, key, offset, length)
		return e
	})
	return data, err
}

// Delete removes an object with retry logic.
func (r *RetryableBackend) Delete(ctx context.Context, key string) error {
	return r.retry(ctx, "delete", func() error {
		return r.backend.Delete(ctx, key)
	})
}

// List returns objects matching a prefix with retry logic.
func (r *RetryableBackend) List(ctx context.Context, prefix string, limit int) ([]ObjectInfo, error) {
	var results []ObjectInfo
	err := r.retry(ctx, "list", func() error {
		var e error
		results, e = r.backend.List(ctx, prefix, limit)
		return e
	})
	return results, err
}

// Copy duplicates an object with retry logic.
func (r *RetryableBackend) Copy(ctx context.Context, srcKey, dstKey string) error {
	return r.retry(ctx, "copy", func() error {
		return r.backend.Copy(ctx, srcKey, dstKey)
	})
}

// Exists checks if an object exists with retry logic.
func (r *RetryableBackend) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := r.retry(ctx, "exists", func() error {
		var e error
		exists, e = r.backend.Exists(ctx, key)
		return e
	})
	return exists, err
}

// Close closes the underlying backend.
func (r *RetryableBackend) Close() error {
	return r.backend.Close()
}

package cloudstorage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- LocalBackend tests ---

func TestLocalBackend_PutAndGet(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)
	defer b.Close()

	data := []byte("hello world")
	require.NoError(t, b.Put(ctx, "test/key1", data, "text/plain", map[string]string{"env": "test"}))

	got, info, err := b.Get(ctx, "test/key1")
	require.NoError(t, err)
	assert.Equal(t, data, got)
	assert.Equal(t, "test/key1", info.Key)
	assert.Equal(t, int64(11), info.Size)
	assert.NotEmpty(t, info.ETag)
}

func TestLocalBackend_GetNotFound(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	_, _, err = b.Get(ctx, "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectNotFound))
}

func TestLocalBackend_GetRange(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, b.Put(ctx, "range-key", []byte("abcdefghij"), "text/plain", nil))

	got, err := b.GetRange(ctx, "range-key", 2, 5)
	require.NoError(t, err)
	assert.Equal(t, []byte("cdefg"), got)
}

func TestLocalBackend_GetRange_NotFound(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	_, err = b.GetRange(ctx, "missing", 0, 5)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectNotFound))
}

func TestLocalBackend_Delete(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, b.Put(ctx, "del-key", []byte("data"), "text/plain", nil))
	require.NoError(t, b.Delete(ctx, "del-key"))

	exists, err := b.Exists(ctx, "del-key")
	require.NoError(t, err)
	assert.False(t, exists)

	err = b.Delete(ctx, "del-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectNotFound))
}

func TestLocalBackend_List(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, b.Put(ctx, "images/a.png", []byte("a"), "image/png", nil))
	require.NoError(t, b.Put(ctx, "images/b.png", []byte("b"), "image/png", nil))
	require.NoError(t, b.Put(ctx, "docs/readme.md", []byte("d"), "text/markdown", nil))

	images, err := b.List(ctx, "images/", 100)
	require.NoError(t, err)
	assert.Len(t, images, 2)

	all, err := b.List(ctx, "", 100)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// Verify sorted order
	assert.Equal(t, "docs/readme.md", all[0].Key)
}

func TestLocalBackend_List_WithLimit(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, b.Put(ctx, "a", []byte("1"), "text/plain", nil))
	require.NoError(t, b.Put(ctx, "b", []byte("2"), "text/plain", nil))
	require.NoError(t, b.Put(ctx, "c", []byte("3"), "text/plain", nil))

	results, err := b.List(ctx, "", 2)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 2)
}

func TestLocalBackend_Copy(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, b.Put(ctx, "src", []byte("original"), "text/plain", nil))
	require.NoError(t, b.Copy(ctx, "src", "dst"))

	got, info, err := b.Get(ctx, "dst")
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), got)
	assert.Equal(t, "dst", info.Key)

	exists, err := b.Exists(ctx, "src")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalBackend_Exists(t *testing.T) {
	ctx := context.Background()
	b, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	exists, err := b.Exists(ctx, "nope")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, b.Put(ctx, "yes", []byte("data"), "text/plain", nil))
	exists, err = b.Exists(ctx, "yes")
	require.NoError(t, err)
	assert.True(t, exists)
}

// --- RetryableBackend tests ---

// failingBackend is a test helper that fails a configurable number of times.
type failingBackend struct {
	failures  atomic.Int32
	maxFails  int32
	inner     Backend
}

func (f *failingBackend) Put(ctx context.Context, key string, data []byte, contentType string, metadata map[string]string) error {
	if int(f.failures.Add(1)) <= int(f.maxFails) {
		return errors.New("transient error")
	}
	return f.inner.Put(ctx, key, data, contentType, metadata)
}

func (f *failingBackend) Get(ctx context.Context, key string) ([]byte, *ObjectInfo, error) {
	if int(f.failures.Add(1)) <= int(f.maxFails) {
		return nil, nil, errors.New("transient error")
	}
	return f.inner.Get(ctx, key)
}

func (f *failingBackend) GetRange(ctx context.Context, key string, offset, length int64) ([]byte, error) {
	return f.inner.GetRange(ctx, key, offset, length)
}

func (f *failingBackend) Delete(ctx context.Context, key string) error {
	return f.inner.Delete(ctx, key)
}

func (f *failingBackend) List(ctx context.Context, prefix string, limit int) ([]ObjectInfo, error) {
	return f.inner.List(ctx, prefix, limit)
}

func (f *failingBackend) Copy(ctx context.Context, srcKey, dstKey string) error {
	return f.inner.Copy(ctx, srcKey, dstKey)
}

func (f *failingBackend) Exists(ctx context.Context, key string) (bool, error) {
	return f.inner.Exists(ctx, key)
}

func (f *failingBackend) Close() error { return nil }

func TestRetryableBackend_Success(t *testing.T) {
	ctx := context.Background()
	inner, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	rb := NewRetryableBackend(inner, DefaultRetryConfig())

	require.NoError(t, rb.Put(ctx, "key1", []byte("data"), "text/plain", nil))

	got, info, err := rb.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), got)
	assert.Equal(t, "key1", info.Key)
}

func TestRetryableBackend_RetriesOnFailure(t *testing.T) {
	ctx := context.Background()
	inner, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	fb := &failingBackend{maxFails: 2, inner: inner}
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1,
		MaxBackoff:     10,
		BackoffFactor:  2.0,
	}
	rb := NewRetryableBackend(fb, cfg)

	// Put should succeed after 2 failures
	require.NoError(t, rb.Put(ctx, "key1", []byte("data"), "text/plain", nil))
	assert.Equal(t, int32(3), fb.failures.Load()) // 2 fails + 1 success
}

func TestRetryableBackend_ExhaustsRetries(t *testing.T) {
	ctx := context.Background()
	inner, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	fb := &failingBackend{maxFails: 100, inner: inner}
	cfg := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 1,
		MaxBackoff:     10,
		BackoffFactor:  2.0,
	}
	rb := NewRetryableBackend(fb, cfg)

	_, _, err = rb.Get(ctx, "key1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 2 retries")
}

// --- ColdTier tests ---

func TestColdTier_ArchiveAndPromote(t *testing.T) {
	ctx := context.Background()
	backend, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	cfg := DefaultColdTierConfig()
	cfg.Backend = backend
	ct, err := NewColdTier(cfg)
	require.NoError(t, err)
	defer ct.Close()

	data := []byte("cold data")
	require.NoError(t, ct.Archive(ctx, "feature/v1", data, "application/octet-stream"))

	assert.True(t, ct.IsArchived("feature/v1"))

	got, info, err := ct.Promote(ctx, "feature/v1")
	require.NoError(t, err)
	assert.Equal(t, data, got)
	assert.NotNil(t, info)

	stats := ct.Stats()
	assert.Equal(t, int64(1), stats.TotalArchived)
	assert.Equal(t, int64(1), stats.TotalPromoted)
	assert.Equal(t, int64(9), stats.TotalSizeBytes)
}

func TestColdTier_Delete(t *testing.T) {
	ctx := context.Background()
	backend, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	cfg := DefaultColdTierConfig()
	cfg.Backend = backend
	ct, err := NewColdTier(cfg)
	require.NoError(t, err)
	defer ct.Close()

	require.NoError(t, ct.Archive(ctx, "to-delete", []byte("temp"), "text/plain"))
	assert.True(t, ct.IsArchived("to-delete"))

	require.NoError(t, ct.Delete(ctx, "to-delete"))
	assert.False(t, ct.IsArchived("to-delete"))

	stats := ct.Stats()
	assert.Equal(t, int64(0), stats.TotalSizeBytes)
}

func TestColdTier_ListArchives(t *testing.T) {
	ctx := context.Background()
	backend, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	cfg := DefaultColdTierConfig()
	cfg.Backend = backend
	ct, err := NewColdTier(cfg)
	require.NoError(t, err)
	defer ct.Close()

	require.NoError(t, ct.Archive(ctx, "a", []byte("1"), "text/plain"))
	require.NoError(t, ct.Archive(ctx, "b", []byte("22"), "text/plain"))

	archives := ct.ListArchives()
	assert.Len(t, archives, 2)
}

func TestColdTier_NilBackend(t *testing.T) {
	cfg := DefaultColdTierConfig()
	_, err := NewColdTier(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backend is required")
}

func TestColdTier_PromoteNotFound(t *testing.T) {
	ctx := context.Background()
	backend, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	cfg := DefaultColdTierConfig()
	cfg.Backend = backend
	ct, err := NewColdTier(cfg)
	require.NoError(t, err)
	defer ct.Close()

	_, _, err = ct.Promote(ctx, "nonexistent")
	require.Error(t, err)

	stats := ct.Stats()
	assert.Equal(t, int64(1), stats.PromotionErrors)
}

// --- ObjectStore SetBackend test ---

func TestObjectStore_SetBackend(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())
	backend, err := NewLocalBackend(t.TempDir())
	require.NoError(t, err)

	s.SetBackend(backend)
	assert.NotNil(t, s.backend)
}

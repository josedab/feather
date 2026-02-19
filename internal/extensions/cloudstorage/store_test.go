package cloudstorage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewObjectStore(t *testing.T) {
	cfg := DefaultStoreConfig()
	s := NewObjectStore(cfg)
	require.NotNil(t, s)
	assert.Equal(t, ProviderLocal, s.config.Provider)
	assert.Equal(t, "feather-data", s.config.Bucket)
}

func TestPutAndGet(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())

	data := []byte("hello world")
	require.NoError(t, s.Put("test/key1", data, "text/plain", map[string]string{"env": "test"}))

	got, info, err := s.Get("test/key1")
	require.NoError(t, err)
	assert.Equal(t, data, got)
	assert.Equal(t, "test/key1", info.Key)
	assert.Equal(t, int64(11), info.Size)
	assert.Equal(t, "text/plain", info.ContentType)
	assert.NotEmpty(t, info.ETag)
	assert.Equal(t, "test", info.Metadata["env"])
}

func TestDelete(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())

	require.NoError(t, s.Put("del-key", []byte("data"), "text/plain", nil))
	require.NoError(t, s.Delete("del-key"))
	assert.False(t, s.Exists("del-key"))

	err := s.Delete("del-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectNotFound))
}

func TestList(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())

	require.NoError(t, s.Put("images/a.png", []byte("a"), "image/png", nil))
	require.NoError(t, s.Put("images/b.png", []byte("b"), "image/png", nil))
	require.NoError(t, s.Put("images/c.png", []byte("c"), "image/png", nil))
	require.NoError(t, s.Put("docs/readme.md", []byte("d"), "text/markdown", nil))
	require.NoError(t, s.Put("docs/guide.md", []byte("e"), "text/markdown", nil))

	images := s.List("images/", 100)
	assert.Len(t, images, 3)

	docs := s.List("docs/", 100)
	assert.Len(t, docs, 2)

	all := s.List("", 100)
	assert.Len(t, all, 5)
}

func TestExists(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())

	assert.False(t, s.Exists("nope"))
	require.NoError(t, s.Put("yes", []byte("data"), "text/plain", nil))
	assert.True(t, s.Exists("yes"))
}

func TestCopy(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())

	require.NoError(t, s.Put("src", []byte("original"), "text/plain", map[string]string{"a": "1"}))
	require.NoError(t, s.Copy("src", "dst"))

	data, info, err := s.Get("dst")
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), data)
	assert.Equal(t, "dst", info.Key)
	assert.Equal(t, "1", info.Metadata["a"])

	// source still exists
	assert.True(t, s.Exists("src"))
}

func TestObjectNotFound(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())

	_, _, err := s.Get("missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectNotFound))

	_, err = s.GetInfo("missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectNotFound))
}

func TestStats(t *testing.T) {
	s := NewObjectStore(DefaultStoreConfig())

	require.NoError(t, s.Put("k1", []byte("aaa"), "text/plain", nil))
	require.NoError(t, s.Put("k2", []byte("bbbbb"), "text/plain", nil))
	_, _, _ = s.Get("k1")
	_ = s.Delete("k2")

	stats := s.Stats()
	assert.Equal(t, int64(2), stats.TotalPuts)
	assert.Equal(t, int64(1), stats.TotalGets)
	assert.Equal(t, int64(1), stats.TotalDeletes)
	assert.Equal(t, 1, stats.TotalObjects)
	assert.Equal(t, "local", stats.Provider)
}
